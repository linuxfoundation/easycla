// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// fetchConcurrency caps the external calls one listing keeps in flight per stage
const fetchConcurrency = 8

// claRef is one returned row before resolution: the user record it belongs to and its signature
type claRef struct {
	user *v1Models.User
	sig  *signatures.ItemSignature
}

// eclaRef is one distinct (CLA Group, employer) pair and the user records holding an ECLA under it
type eclaRef struct {
	users      []*v1Models.User
	claGroupID string
	companyID  string
}

// claData is everything the rows need from DynamoDB, the projects service, the organizations
// service, the sanctions screen and GitHub - resolved once per distinct key, concurrently
type claData struct {
	claGroupNames map[string]string
	projectInfos  map[string]projectInfo
	companies     map[string]*v1Models.Company
	sanctions     map[string]sanctionState
	cclas         map[string]*v1Models.Signature
	approvals     map[string]eclaCoverage
}

func cclaKey(claGroupID, companyID string) string {
	return claGroupID + "|" + companyID
}

func approvalKey(claGroupID, companyID, userID string) string {
	return cclaKey(claGroupID, companyID) + "|" + userID
}

// userSignatures loads every user record's signatures concurrently, keeping the order of
// userModels so the resulting listing stays deterministic
func (s *service) userSignatures(ctx context.Context, userModels []*v1Models.User) ([][]*signatures.ItemSignature, error) {
	perUser := make([][]*signatures.ItemSignature, len(userModels))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(fetchConcurrency)
	for i, userModel := range userModels {
		group.Go(func() error {
			userSigs, err := s.repo.GetUserCLASignatures(groupCtx, userModel.UserID)
			if err != nil {
				return err
			}
			perUser[i] = userSigs
			return nil
		})
	}
	return perUser, group.Wait()
}

// claRefs flattens the loaded signatures into the rows to return, dropping the unsigned and the
// duplicates
func claRefs(userModels []*v1Models.User, perUser [][]*signatures.ItemSignature) []claRef {
	seen := make(map[string]bool)
	refs := make([]claRef, 0)
	for i, userModel := range userModels {
		for _, sig := range perUser[i] {
			if !sig.SignatureSigned || seen[sig.SignatureID] {
				continue
			}
			seen[sig.SignatureID] = true
			refs = append(refs, claRef{user: userModel, sig: sig})
		}
	}
	return refs
}

// prefetch resolves every external dependency of the given rows up front, running the three
// independent chains - CLA Group and project details, employer and its sanctions screen,
// corporate signature and its approval-list evaluation - concurrently. Only the CLA Group chain
// can fail the listing: an employer, CCLA or approval-list failure degrades its own rows.
func (s *service) prefetch(ctx context.Context, refs []claRef) (*claData, error) {
	data := &claData{
		claGroupNames: make(map[string]string),
		projectInfos:  make(map[string]projectInfo),
		companies:     make(map[string]*v1Models.Company),
		sanctions:     make(map[string]sanctionState),
		cclas:         make(map[string]*v1Models.Signature),
		approvals:     make(map[string]eclaCoverage),
	}
	claGroupIDs, companyIDs, eclaRefs := distinctRefs(refs)

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return s.loadProjects(groupCtx, data, claGroupIDs) })
	group.Go(func() error { return s.loadEmployers(groupCtx, data, companyIDs) })
	group.Go(func() error { return s.loadCoverage(groupCtx, data, eclaRefs) })
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return data, nil
}

// distinctRefs collects the keys to resolve: one per CLA Group, one per employer and one per
// (CLA Group, employer) pair carrying the user records that pair must be evaluated for
func distinctRefs(refs []claRef) ([]string, []string, []eclaRef) {
	var claGroupIDs, companyIDs []string
	var eclaRefs []eclaRef
	seenClaGroup := make(map[string]bool)
	seenCompany := make(map[string]bool)
	seenEcla := make(map[string]int)
	seenEclaUser := make(map[string]bool)
	for _, ref := range refs {
		claGroupID := ref.sig.SignatureProjectID
		if claGroupID != "" && !seenClaGroup[claGroupID] {
			seenClaGroup[claGroupID] = true
			claGroupIDs = append(claGroupIDs, claGroupID)
		}
		companyID := ref.sig.SignatureUserCompanyID
		if companyID == "" {
			continue
		}
		if !seenCompany[companyID] {
			seenCompany[companyID] = true
			companyIDs = append(companyIDs, companyID)
		}
		key := cclaKey(claGroupID, companyID)
		index, ok := seenEcla[key]
		if !ok {
			index = len(eclaRefs)
			seenEcla[key] = index
			eclaRefs = append(eclaRefs, eclaRef{claGroupID: claGroupID, companyID: companyID})
		}
		if userKey := approvalKey(claGroupID, companyID, ref.user.UserID); !seenEclaUser[userKey] {
			seenEclaUser[userKey] = true
			eclaRefs[index].users = append(eclaRefs[index].users, ref.user)
		}
	}
	return claGroupIDs, companyIDs, eclaRefs
}

// loadProjects resolves the CLA Group name and the project name and logo of each distinct CLA
// Group. These failures affect every row alike, so they fail the listing rather than degrade it.
func (s *service) loadProjects(ctx context.Context, data *claData, claGroupIDs []string) error {
	names := make([]string, len(claGroupIDs))
	infos := make([]projectInfo, len(claGroupIDs))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(fetchConcurrency)
	for i, claGroupID := range claGroupIDs {
		group.Go(func() error {
			name, err := s.claGroupName(groupCtx, claGroupID)
			if err != nil {
				return err
			}
			names[i] = name
			return nil
		})
		group.Go(func() error {
			info, err := s.projectInfo(groupCtx, claGroupID)
			if err != nil {
				return err
			}
			infos[i] = info
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	for i, claGroupID := range claGroupIDs {
		data.claGroupNames[claGroupID] = names[i]
		data.projectInfos[claGroupID] = infos[i]
	}
	return nil
}

// loadEmployers looks up each distinct employer and screens it for sanctions in the same chain,
// so a slow screen never delays another employer. A failed lookup degrades that employer's rows.
func (s *service) loadEmployers(ctx context.Context, data *claData, companyIDs []string) error {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.prefetch.loadEmployers",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	companyModels := make([]*v1Models.Company, len(companyIDs))
	states := make([]sanctionState, len(companyIDs))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(fetchConcurrency)
	for i, companyID := range companyIDs {
		group.Go(func() error {
			companyModel, err := s.company(groupCtx, companyID)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("unable to lookup employer %s - degrading its rows", companyID)
				return nil
			}
			companyModels[i] = companyModel
			states[i] = s.companySanctions(groupCtx, companyModel)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	for i, companyID := range companyIDs {
		data.companies[companyID] = companyModels[i]
		data.sanctions[companyID] = states[i]
	}
	return nil
}

// loadCoverage resolves the corporate signature of each distinct (CLA Group, employer) pair and
// then evaluates its approval lists for every user record holding an ECLA under it. Both stages
// run concurrently within themselves; a failure degrades the affected rows only.
func (s *service) loadCoverage(ctx context.Context, data *claData, eclaRefs []eclaRef) error {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.prefetch.loadCoverage",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	cclas := make([]*v1Models.Signature, len(eclaRefs))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(fetchConcurrency)
	for i, ref := range eclaRefs {
		group.Go(func() error {
			approved, signed := true, true
			ccla, err := s.signaturesService.GetCorporateSignature(groupCtx, ref.claGroupID, ref.companyID, &approved, &signed)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("unable to lookup the corporate signature of %s for CLA Group %s - degrading its rows", ref.companyID, ref.claGroupID)
				return nil
			}
			cclas[i] = ccla
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}

	type approvalRef struct {
		ccla *v1Models.Signature
		user *v1Models.User
		key  string
	}
	var approvalRefs []approvalRef
	for i, ref := range eclaRefs {
		data.cclas[cclaKey(ref.claGroupID, ref.companyID)] = cclas[i]
		if cclas[i] == nil {
			continue
		}
		for _, userModel := range ref.users {
			approvalRefs = append(approvalRefs, approvalRef{
				key:  approvalKey(ref.claGroupID, ref.companyID, userModel.UserID),
				user: userModel,
				ccla: cclas[i],
			})
		}
	}

	coverages := make([]eclaCoverage, len(approvalRefs))
	approvalGroup, approvalCtx := errgroup.WithContext(ctx)
	approvalGroup.SetLimit(fetchConcurrency)
	for i, ref := range approvalRefs {
		approvalGroup.Go(func() error {
			coverages[i] = s.evaluateApproval(approvalCtx, ref.user, ref.ccla)
			return nil
		})
	}
	if err := approvalGroup.Wait(); err != nil {
		return err
	}
	for i, ref := range approvalRefs {
		data.approvals[ref.key] = coverages[i]
	}
	return nil
}

// coverage is the resolved approval-list outcome of one ECLA row. An unreadable employer, a
// sanctioned employer, a missing or unreadable corporate signature and a failed approval-list
// check are all unevaluable, so a false covered never means "no longer approved".
func (d *claData) coverage(sig *signatures.ItemSignature, userModel *v1Models.User, flagged bool) eclaCoverage {
	if flagged || d.companies[sig.SignatureUserCompanyID] == nil {
		return eclaCoverage{unevaluable: true}
	}
	if d.cclas[cclaKey(sig.SignatureProjectID, sig.SignatureUserCompanyID)] == nil {
		return eclaCoverage{unevaluable: true}
	}
	if resolved, ok := d.approvals[approvalKey(sig.SignatureProjectID, sig.SignatureUserCompanyID, userModel.UserID)]; ok {
		return resolved
	}
	return eclaCoverage{unevaluable: true}
}

// claManager reports the caller as a CLA manager of the employer only on rows whose coverage was
// evaluated: a revoked or unreadable-employer row carries no action
func (d *claData) claManager(sig *signatures.ItemSignature, lfUsername string, flagged bool) bool {
	if flagged || d.companies[sig.SignatureUserCompanyID] == nil {
		return false
	}
	return isClaManager(d.cclas[cclaKey(sig.SignatureProjectID, sig.SignatureUserCompanyID)], lfUsername)
}
