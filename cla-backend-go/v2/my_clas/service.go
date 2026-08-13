// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	v2ProjectServiceModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service/models"
	platformModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
	"github.com/sirupsen/logrus"
)

const (
	identitySourceGithub       = "github"
	identitySourceGitlab       = "gitlab"
	identitySourceGerrit       = "gerrit"
	identityDataSourcePlatform = "platform"
)

// Identity holds the caller-provided identity keys used to resolve EasyCLA user records
type Identity struct {
	LfUsername      string
	Emails          []string
	SecondaryEmails []string
	GithubIDs       []int64
	GithubUsernames []string
	GitlabIDs       []int64
	GitlabUsernames []string
	GerritUsernames []string
}

// IsEmpty returns true when no non-blank identity key was provided
func (i *Identity) IsEmpty() bool {
	return strings.TrimSpace(i.LfUsername) == "" && len(i.GithubIDs) == 0 && len(i.GitlabIDs) == 0 &&
		!hasValue(i.Emails) && !hasValue(i.SecondaryEmails) && !hasValue(i.GithubUsernames) &&
		!hasValue(i.GitlabUsernames) && !hasValue(i.GerritUsernames)
}

func hasValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

// PlatformUsersService is the subset of the platform user-service client used to verify
// that identities are connected to the authenticated user's LF account
type PlatformUsersService interface {
	GetUserByUsernameContext(ctx context.Context, lfUsername string) (*platformModels.User, error)
	ListUserIdentities(ctx context.Context, userSFID string) ([]*platformModels.UserIdentity, error)
}

// SignaturesService is the subset of the v1 signatures service used to evaluate ECLA validity
type SignaturesService interface {
	GetCorporateSignature(ctx context.Context, claGroupID, companyID string, approved, signed *bool) (*v1Models.Signature, error)
	EvaluateUserApproval(ctx context.Context, user *v1Models.User, cclaSignature *v1Models.Signature) (approved bool, githubOrgLookupFailed bool, err error)
}

// CompanyRepository is the subset of the company repository used to resolve employers
type CompanyRepository interface {
	GetCompany(ctx context.Context, companyID string) (*v1Models.Company, error)
}

// ProjectsCLAGroupsRepository is the subset of the projects-cla-groups repository used to resolve
// CLA Group names and the Salesforce project(s) a CLA Group is mapped to
type ProjectsCLAGroupsRepository interface {
	GetCLAGroupNameByID(ctx context.Context, claGroupID string) (string, error)
	GetProjectsIdsForClaGroup(ctx context.Context, claGroupID string) ([]*projects_cla_groups.ProjectClaGroup, error)
}

// ProjectService is the subset of the project-service client used to resolve a project's
// display name and logo from its Salesforce ID
type ProjectService interface {
	GetProject(projectSFID string) (*v2ProjectServiceModels.ProjectOutputDetailed, error)
}

// Service interface defines the My CLAs service methods
type Service interface {
	GetMyClas(ctx context.Context, currentUsername string, admin bool, requested *Identity) (*models.MyClaList, error)
	GetMyClaPdfURL(ctx context.Context, currentUsername string, admin bool, requested *Identity, signatureID string) (*models.MyClaPdf, error)
	GetMyIdentities(ctx context.Context, currentUsername string) (*models.MyIdentityList, error)
}

type service struct {
	repo                  Repository
	platformUsersService  PlatformUsersService
	signaturesService     SignaturesService
	companyRepo           CompanyRepository
	projectsClaGroupsRepo ProjectsCLAGroupsRepository
	projectService        ProjectService
	presign               func(filename string) (string, error)
	documentExists        func(filename string) (bool, error)
}

// NewService creates a new instance of the My CLAs service
func NewService(repo Repository, platformUsersService PlatformUsersService, signaturesService SignaturesService, companyRepo CompanyRepository, projectsClaGroupsRepo ProjectsCLAGroupsRepository, projectService ProjectService) Service {
	return &service{
		repo:                  repo,
		platformUsersService:  platformUsersService,
		signaturesService:     signaturesService,
		companyRepo:           companyRepo,
		projectsClaGroupsRepo: projectsClaGroupsRepo,
		projectService:        projectService,
		presign:               utils.GetDownloadLink,
		documentExists:        utils.DocumentExists,
	}
}

// projectInfo holds the resolved Salesforce project display name and logo for a CLA Group
type projectInfo struct {
	name string
	logo string
}

// GetMyClas returns all signed ICLAs and ECLAs of the EasyCLA user records matching the
// given identity, with validity evaluated against the current CCLA approval lists
func (s *service) GetMyClas(ctx context.Context, currentUsername string, admin bool, requested *Identity) (*models.MyClaList, error) {
	f := logrus.Fields{
		"functionName":    "v2.my_clas.service.GetMyClas",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"currentUsername": currentUsername,
		"admin":           admin,
	}

	identity, skipped, err := s.effectiveIdentity(ctx, currentUsername, admin, requested)
	if err != nil {
		return nil, err
	}

	userModels, err := s.resolveUsers(ctx, identity)
	if err != nil {
		return nil, err
	}

	result := &models.MyClaList{
		LfUsername:        identity.LfUsername,
		UserIds:           make([]string, 0, len(userModels)),
		SkippedIdentities: skipped,
		Clas:              []models.MyCla{},
	}

	seen := make(map[string]bool)
	claGroupNames := make(map[string]string)
	projectInfos := make(map[string]projectInfo)
	companies := make(map[string]*v1Models.Company)
	cclas := make(map[string]*v1Models.Signature)
	approvals := make(map[string]eclaCoverage)

	for _, userModel := range userModels {
		result.UserIds = append(result.UserIds, userModel.UserID)

		userSignatures, sigErr := s.repo.GetUserCLASignatures(ctx, userModel.UserID)
		if sigErr != nil {
			return nil, sigErr
		}

		for _, sig := range userSignatures {
			if !sig.SignatureSigned || seen[sig.SignatureID] {
				continue
			}
			seen[sig.SignatureID] = true

			claGroupName, nameErr := s.claGroupName(ctx, claGroupNames, sig.SignatureProjectID)
			if nameErr != nil {
				return nil, nameErr
			}
			project, projectErr := s.projectInfo(ctx, projectInfos, sig.SignatureProjectID)
			if projectErr != nil {
				return nil, projectErr
			}
			row := models.MyCla{
				SignatureID:          sig.SignatureID,
				ClaGroupID:           sig.SignatureProjectID,
				ClaGroupName:         claGroupName,
				ProjectName:          project.name,
				ProjectLogo:          project.logo,
				UserID:               sig.SignatureReferenceID,
				SignedOn:             signedOn(sig),
				Signed:               sig.SignatureSigned,
				Approved:             sig.SignatureApproved,
				DocumentMajorVersion: int64(sig.SignatureDocumentMajorVersion),
				DocumentMinorVersion: int64(sig.SignatureDocumentMinorVersion),
			}

			if sig.SignatureUserCompanyID == "" {
				row.ClaType = utils.ClaTypeICLA
				row.Valid = sig.SignatureApproved
				row.PdfAvailable = true
				assignMyClaStatus(&row, false, false)
			} else {
				row.ClaType = utils.ClaTypeECLA
				row.CompanyID = sig.SignatureUserCompanyID
				companyModel, companyErr := s.company(ctx, companies, sig.SignatureUserCompanyID)
				if companyErr != nil {
					log.WithFields(f).WithError(companyErr).Warn("unable to lookup the employer for the employee acknowledgement - degrading the row")
					companyModel = nil
				}
				if companyModel != nil {
					row.CompanyName = companyModel.CompanyName
					row.SigningEntityName = companyModel.SigningEntityName
				}
				covered, unevaluable := s.eclaCoveredByCurrentApprovalList(ctx, cclas, approvals, userModel, companyModel, sig)
				row.Valid = sig.SignatureApproved && covered
				assignMyClaStatus(&row, covered, unevaluable)
			}

			result.Clas = append(result.Clas, row)
		}
	}

	sort.SliceStable(result.Clas, func(i, j int) bool {
		return result.Clas[i].SignedOn > result.Clas[j].SignedOn
	})
	result.ResultCount = int64(len(result.Clas))

	log.WithFields(f).Debugf("resolved %d user records with %d CLA records (%d identity keys skipped)", len(result.UserIds), result.ResultCount, len(skipped))
	return result, nil
}

// GetMyClaPdfURL returns a time-limited download URL for the signed ICLA PDF when the
// signature belongs to one of the EasyCLA user records matching the given identity -
// a nil result means unknown, not-owned, unsigned or ECLA signature ID
func (s *service) GetMyClaPdfURL(ctx context.Context, currentUsername string, admin bool, requested *Identity, signatureID string) (*models.MyClaPdf, error) {
	f := logrus.Fields{
		"functionName":    "v2.my_clas.service.GetMyClaPdfURL",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"currentUsername": currentUsername,
		"admin":           admin,
		"signatureID":     signatureID,
	}

	identity, _, err := s.effectiveIdentity(ctx, currentUsername, admin, requested)
	if err != nil {
		return nil, err
	}

	userModels, err := s.resolveUsers(ctx, identity)
	if err != nil {
		return nil, err
	}

	for _, userModel := range userModels {
		userSignatures, sigErr := s.repo.GetUserCLASignatures(ctx, userModel.UserID)
		if sigErr != nil {
			return nil, sigErr
		}

		for _, sig := range userSignatures {
			if sig.SignatureID != signatureID {
				continue
			}
			if !sig.SignatureSigned || sig.SignatureUserCompanyID != "" {
				log.WithFields(f).Debug("signature is not a signed ICLA - no PDF available")
				return nil, nil
			}

			filename := utils.SignedCLAFilename(sig.SignatureProjectID, utils.ClaTypeICLA, sig.SignatureReferenceID, sig.SignatureID)
			exists, existsErr := s.documentExists(filename)
			if existsErr != nil {
				log.WithFields(f).WithError(existsErr).Warnf("unable to check the signed document existence for file: %s", filename)
				return nil, existsErr
			}
			if !exists {
				log.WithFields(f).Warnf("signed document does not exist in S3: %s", filename)
				return nil, nil
			}
			url, urlErr := s.presign(filename)
			if urlErr != nil {
				log.WithFields(f).WithError(urlErr).Warnf("unable to generate a download link for file: %s", filename)
				return nil, urlErr
			}

			return &models.MyClaPdf{
				SignatureID:      sig.SignatureID,
				URL:              url,
				ExpiresInSeconds: int64(utils.PresignedURLValidity.Seconds()),
			}, nil
		}
	}

	return nil, nil
}

// GetMyIdentities returns the deduplicated "<type>:<value>" identities the authenticated user
// owns - the union of their EasyCLA user records and their platform user-service account, the
// same two sources authorizeIdentity uses to authorize a non-admin caller's identity keys
func (s *service) GetMyIdentities(ctx context.Context, currentUsername string) (*models.MyIdentityList, error) {
	if currentUsername == "" {
		return nil, errors.New("no username on the authenticated principal")
	}

	selfUsers, err := s.repo.GetUsersByLFUsername(ctx, currentUsername)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	identities := []string{}
	add := func(kind, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		entry := kind + ":" + value
		if seen[entry] {
			return
		}
		seen[entry] = true
		identities = append(identities, entry)
	}

	add("lf-username", currentUsername)
	for _, selfUser := range selfUsers {
		add("lf-username", selfUser.LfUsername)
		add("email", normalizeEmail(string(selfUser.LfEmail)))
		for _, email := range selfUser.Emails {
			add("email", normalizeEmail(email))
		}
		add("github-id", selfUser.GithubID)
		add("github-username", selfUser.GithubUsername)
		add("gitlab-id", selfUser.GitlabID)
		add("gitlab-username", selfUser.GitlabUsername)
	}

	platform := s.loadPlatformIdentities(ctx, currentUsername)
	for email := range platform.emails {
		add("email", email)
	}
	for source, byKey := range platform.usernames {
		for _, variants := range byKey {
			for _, variant := range variants {
				add(source+"-username", variant)
			}
		}
	}

	sort.Strings(identities)
	return &models.MyIdentityList{
		LfUsername:  currentUsername,
		Identities:  identities,
		ResultCount: int64(len(identities)),
	}, nil
}

func (s *service) effectiveIdentity(ctx context.Context, currentUsername string, admin bool, requested *Identity) (*Identity, []string, error) {
	if admin {
		identity := *requested
		if identity.LfUsername == "" {
			identity.LfUsername = currentUsername
		}
		return &identity, []string{}, nil
	}
	if currentUsername == "" {
		return nil, nil, errors.New("no username on the authenticated principal")
	}
	return s.authorizeIdentity(ctx, currentUsername, requested)
}

type platformIdentitySet struct {
	emails    map[string]bool
	usernames map[string]map[string][]string
}

// authorizeIdentity verifies each requested identity key against all of the
// authenticated user's own EasyCLA records and, when not covered there, against the
// identities connected to their LF account in the platform user-service - unverified
// keys are dropped from the search and reported back; verified usernames are replaced
// by their canonical spellings for the exact-match index lookups
func (s *service) authorizeIdentity(ctx context.Context, currentUsername string, requested *Identity) (*Identity, []string, error) {
	f := logrus.Fields{
		"functionName":    "v2.my_clas.service.authorizeIdentity",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"currentUsername": currentUsername,
	}

	selfUsers, err := s.repo.GetUsersByLFUsername(ctx, currentUsername)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup the authenticated user's EasyCLA records")
		return nil, nil, err
	}

	selfEmails := make(map[string]bool)
	selfGithubIDs := make(map[string]bool)
	selfGitlabIDs := make(map[string]bool)
	selfUsernames := map[string]map[string][]string{
		identitySourceGithub: {},
		identitySourceGitlab: {},
		identitySourceGerrit: {strings.ToLower(currentUsername): {currentUsername}},
	}
	for _, selfUser := range selfUsers {
		if selfUser.LfEmail != "" {
			selfEmails[normalizeEmail(string(selfUser.LfEmail))] = true
		}
		for _, email := range selfUser.Emails {
			if email != "" {
				selfEmails[normalizeEmail(email)] = true
			}
		}
		if id := strings.TrimSpace(selfUser.GithubID); id != "" {
			selfGithubIDs[id] = true
		}
		if id := strings.TrimSpace(selfUser.GitlabID); id != "" {
			selfGitlabIDs[id] = true
		}
		addCanonical(selfUsernames[identitySourceGithub], selfUser.GithubUsername)
		addCanonical(selfUsernames[identitySourceGitlab], selfUser.GitlabUsername)
	}

	var platform *platformIdentitySet
	platformIdentities := func() *platformIdentitySet {
		if platform == nil {
			platform = s.loadPlatformIdentities(ctx, currentUsername)
		}
		return platform
	}
	emailAllowed := func(email string) bool {
		return selfEmails[email] || platformIdentities().emails[email]
	}
	canonFor := func(source string) func(string) []string {
		return func(username string) []string {
			key := strings.ToLower(username)
			variants := append([]string{}, selfUsernames[source][key]...)
			variants = append(variants, platformIdentities().usernames[source][key]...)
			return variants
		}
	}
	idAllowed := func(selfIDs map[string]bool) func(int64) bool {
		return func(id int64) bool {
			return selfIDs[strconv.FormatInt(id, 10)]
		}
	}

	allowed := &Identity{LfUsername: currentUsername}
	skipped := []string{}

	if requested.LfUsername != "" && !strings.EqualFold(requested.LfUsername, currentUsername) {
		skipped = append(skipped, "lfUsername:"+requested.LfUsername)
	}
	appendAllowedStrings(requested.Emails, "email", normalizeEmail, emailAllowed, &allowed.Emails, &skipped)
	appendAllowedStrings(requested.SecondaryEmails, "secondaryEmail", normalizeEmail, emailAllowed, &allowed.SecondaryEmails, &skipped)
	appendAllowedIDs(requested.GithubIDs, "githubId", idAllowed(selfGithubIDs), &allowed.GithubIDs, &skipped)
	appendAllowedUsernames(requested.GithubUsernames, "githubUsername", canonFor(identitySourceGithub), &allowed.GithubUsernames, &skipped)
	appendAllowedIDs(requested.GitlabIDs, "gitlabId", idAllowed(selfGitlabIDs), &allowed.GitlabIDs, &skipped)
	appendAllowedUsernames(requested.GitlabUsernames, "gitlabUsername", canonFor(identitySourceGitlab), &allowed.GitlabUsernames, &skipped)
	appendAllowedUsernames(requested.GerritUsernames, "gerritUsername", canonFor(identitySourceGerrit), &allowed.GerritUsernames, &skipped)

	return allowed, skipped, nil
}

func addCanonical(canon map[string][]string, username string) {
	username = strings.TrimSpace(username)
	if username == "" {
		return
	}
	key := strings.ToLower(username)
	for _, existing := range canon[key] {
		if existing == username {
			return
		}
	}
	canon[key] = append(canon[key], username)
}

func appendAllowedStrings(values []string, param string, normalize func(string) string, allowed func(string) bool, dst *[]string, skipped *[]string) {
	seen := make(map[string]bool)
	for _, value := range values {
		value = normalize(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		if allowed(value) {
			*dst = append(*dst, value)
		} else {
			*skipped = append(*skipped, param+":"+value)
		}
	}
}

func appendAllowedIDs(values []int64, param string, allowed func(int64) bool, dst *[]int64, skipped *[]string) {
	seen := make(map[int64]bool)
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		if allowed(value) {
			*dst = append(*dst, value)
		} else {
			*skipped = append(*skipped, fmt.Sprintf("%s:%d", param, value))
		}
	}
}

func appendAllowedUsernames(values []string, param string, canon func(string) []string, dst *[]string, skipped *[]string) {
	seen := make(map[string]bool)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[strings.ToLower(value)] {
			continue
		}
		seen[strings.ToLower(value)] = true
		if variants := canon(value); len(variants) > 0 {
			*dst = append(*dst, append(variants, value)...)
		} else {
			*skipped = append(*skipped, param+":"+value)
		}
	}
}

// loadPlatformIdentities collects the emails and per-source canonical usernames
// connected to the LF account - lookup failures yield an empty set, so the affected
// keys are skipped, never allowed
func (s *service) loadPlatformIdentities(ctx context.Context, lfUsername string) *platformIdentitySet {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.service.loadPlatformIdentities",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"lfUsername":     lfUsername,
	}

	set := &platformIdentitySet{
		emails: make(map[string]bool),
		usernames: map[string]map[string][]string{
			identitySourceGithub: {},
			identitySourceGitlab: {},
			identitySourceGerrit: {},
		},
	}

	if s.platformUsersService == nil {
		return set
	}

	platformUser, err := s.platformUsersService.GetUserByUsernameContext(ctx, lfUsername)
	if err != nil || platformUser == nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup the LF user in the platform user-service")
		return set
	}

	if platformUser.Email != nil && *platformUser.Email != "" {
		set.emails[normalizeEmail(*platformUser.Email)] = true
	}
	for _, email := range platformUser.Emails {
		if email == nil || email.EmailAddress == nil || *email.EmailAddress == "" {
			continue
		}
		if email.IsDeleted != nil && *email.IsDeleted {
			continue
		}
		set.emails[normalizeEmail(*email.EmailAddress)] = true
	}

	if platformUser.ID == "" {
		return set
	}
	identityList, err := s.platformUsersService.ListUserIdentities(ctx, platformUser.ID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to list the LF user's connected identities")
		return set
	}
	for _, identity := range identityList {
		if identity == nil {
			continue
		}
		if identity.DataSource != "" && !strings.EqualFold(identity.DataSource, identityDataSourcePlatform) {
			continue
		}
		source := strings.ToLower(strings.TrimSpace(identity.Source))
		if canon, ok := set.usernames[source]; ok {
			addCanonical(canon, identity.Username)
		}
		if identity.Email != "" {
			set.emails[normalizeEmail(identity.Email)] = true
		}
	}

	return set
}

func (s *service) resolveUsers(ctx context.Context, identity *Identity) ([]*v1Models.User, error) {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.service.resolveUsers",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	var userModels []*v1Models.User
	seen := make(map[string]bool)
	add := func(matches ...*v1Models.User) {
		for _, userModel := range matches {
			if userModel == nil || userModel.UserID == "" || seen[userModel.UserID] {
				continue
			}
			seen[userModel.UserID] = true
			userModels = append(userModels, userModel)
		}
	}
	addByLookup := func(values []string, what string, lookup func(context.Context, string) ([]*v1Models.User, error)) error {
		for _, value := range values {
			matches, err := lookup(ctx, value)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("unable to lookup users by %s: %s", what, value)
				return err
			}
			add(matches...)
		}
		return nil
	}
	addByIDLookup := func(ids []int64, what string, lookup func(context.Context, int64) ([]*v1Models.User, error)) error {
		for _, id := range dedupeIDs(ids) {
			matches, err := lookup(ctx, id)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("unable to lookup users by %s: %d", what, id)
				return err
			}
			add(matches...)
		}
		return nil
	}

	lfUsernames := trimAll(append([]string{identity.LfUsername}, identity.GerritUsernames...))
	if err := addByLookup(lfUsernames, "LF username", s.repo.GetUsersByLFUsername); err != nil {
		return nil, err
	}
	if err := addByLookup(normalizeEmails(identity.Emails), "email", s.repo.GetUsersByPrimaryEmail); err != nil {
		return nil, err
	}
	if secondaryEmails := normalizeEmails(identity.SecondaryEmails); len(secondaryEmails) > 0 {
		matches, err := s.repo.GetUsersBySecondaryEmails(ctx, secondaryEmails)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to lookup users by secondary emails")
			return nil, err
		}
		add(matches...)
	}
	if err := addByIDLookup(identity.GithubIDs, "GitHub ID", s.repo.GetUsersByGithubID); err != nil {
		return nil, err
	}
	if err := addByLookup(trimAll(identity.GithubUsernames), "GitHub username", s.repo.GetUsersByGithubUsername); err != nil {
		return nil, err
	}
	if err := addByIDLookup(identity.GitlabIDs, "GitLab ID", s.repo.GetUsersByGitlabID); err != nil {
		return nil, err
	}
	if err := addByLookup(trimAll(identity.GitlabUsernames), "GitLab username", s.repo.GetUsersByGitlabUsername); err != nil {
		return nil, err
	}

	return userModels, nil
}

// eclaCoverage is the listing-internal coverage outcome. covered still drives
// `valid`; unevaluable means a genuine approval-list check did not complete.
type eclaCoverage struct {
	covered     bool
	unevaluable bool
}

// assignMyClaStatus sets status / statusReason independently of approved / valid.
// ICLA is binary. ECLA needs_attention only after a completed list miss.
func assignMyClaStatus(row *models.MyCla, covered, unevaluable bool) {
	if row.ClaType == utils.ClaTypeICLA {
		if row.Approved {
			row.Status = models.MyClaStatusValid
		} else {
			row.Status = models.MyClaStatusInvalidated
		}
		return
	}
	if !row.Approved {
		row.Status = models.MyClaStatusInvalidated
		return
	}
	if unevaluable {
		row.Status = models.MyClaStatusUnknown
		row.StatusReason = models.MyClaStatusReasonUnknown
		return
	}
	if covered {
		row.Status = models.MyClaStatusValid
		return
	}
	row.Status = models.MyClaStatusNeedsAttention
	row.StatusReason = models.MyClaStatusReasonNotOnApprovalList
}

// eclaCoveredByCurrentApprovalList mirrors the PR gating logic (signatures service
// ProcessEmployeeSignature / EvaluateUserApproval): the company must not be sanctioned, must
// hold an approved+signed CCLA for the CLA Group, and the user must match its current
// approval lists
func (s *service) eclaCoveredByCurrentApprovalList(ctx context.Context, cclas map[string]*v1Models.Signature, approvals map[string]eclaCoverage, userModel *v1Models.User, companyModel *v1Models.Company, sig *signatures.ItemSignature) (bool, bool) {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.service.eclaCoveredByCurrentApprovalList",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    sig.SignatureID,
		"claGroupID":     sig.SignatureProjectID,
		"companyID":      sig.SignatureUserCompanyID,
	}

	if companyModel == nil || companyModel.IsSanctioned {
		return false, true
	}

	cclaKey := sig.SignatureProjectID + "|" + sig.SignatureUserCompanyID
	ccla, ok := cclas[cclaKey]
	if !ok {
		approved, signed := true, true
		cclaModel, cclaErr := s.signaturesService.GetCorporateSignature(ctx, sig.SignatureProjectID, sig.SignatureUserCompanyID, &approved, &signed)
		if cclaErr != nil {
			log.WithFields(f).WithError(cclaErr).Warn("unable to lookup the corporate signature for the employee acknowledgement")
			cclas[cclaKey] = nil
			return false, true
		}
		ccla = cclaModel
		cclas[cclaKey] = ccla
	}
	if ccla == nil {
		return false, true
	}

	approvalKey := cclaKey + "|" + userModel.UserID
	if cached, ok := approvals[approvalKey]; ok {
		return cached.covered, cached.unevaluable
	}
	covered, githubOrgLookupFailed, approvedErr := s.signaturesService.EvaluateUserApproval(ctx, userModel, ccla)
	unevaluable := false
	if approvedErr != nil {
		log.WithFields(f).WithError(approvedErr).Warn("unable to evaluate the approval list for the employee acknowledgement")
		covered = false
		unevaluable = true
	}
	if githubOrgLookupFailed {
		unevaluable = true
	}
	// EvaluateUserApproval cannot check GitLab group membership (that needs
	// per-group OAuth tokens); defer to the signature_approved flag, which the
	// approval-list invalidation flow maintains (see docs/MY_CLAS_API.md).
	// Membership was not actually evaluated, so the row stays unevaluable for status.
	if !covered && approvedErr == nil && len(ccla.GitlabOrgApprovalList) > 0 {
		covered = true
		unevaluable = true
	}
	approvals[approvalKey] = eclaCoverage{covered: covered, unevaluable: unevaluable}
	return covered, unevaluable
}

func (s *service) claGroupName(ctx context.Context, cache map[string]string, claGroupID string) (string, error) {
	if claGroupID == "" {
		return "", nil
	}
	if name, ok := cache[claGroupID]; ok {
		return name, nil
	}
	name, err := s.projectsClaGroupsRepo.GetCLAGroupNameByID(ctx, claGroupID)
	if err != nil {
		if !errors.Is(err, projects_cla_groups.ErrCLAGroupDoesNotExist) {
			return "", err
		}
		name = ""
	}
	cache[claGroupID] = name
	return name, nil
}

// projectInfo resolves the Salesforce project display name and logo the CLA Group belongs to,
// cached per request. The name comes from the projects-cla-groups mapping table; the logo lives
// only in the project-service and is fetched by project SFID (a foundation-level CLA Group
// resolves to its foundation). A project-service lookup miss degrades to an empty logo rather
// than failing the whole listing.
func (s *service) projectInfo(ctx context.Context, cache map[string]projectInfo, claGroupID string) (projectInfo, error) {
	if claGroupID == "" {
		return projectInfo{}, nil
	}
	if info, ok := cache[claGroupID]; ok {
		return info, nil
	}

	mappings, err := s.projectsClaGroupsRepo.GetProjectsIdsForClaGroup(ctx, claGroupID)
	if err != nil {
		return projectInfo{}, err
	}

	var info projectInfo
	var projectSFID string
	// Foundation-level CLA Groups are identified by a mapping whose ProjectSFID == FoundationSFID
	// (the projects_cla_groups convention used by SignedAtFoundation), NOT by the number of
	// mappings: such a group resolves to its foundation, and a single project-level mapping
	// resolves to that project. Multiple project-level mappings with no foundation marker are
	// left unresolved (empty name/logo, so the consumer falls back to claGroupName) rather than
	// inventing an association with an arbitrary one of the mapped projects.
	switch fm := foundationMapping(mappings); {
	case fm != nil:
		projectSFID = fm.FoundationSFID
		info.name = fm.FoundationName
	case len(mappings) == 1:
		projectSFID = mappings[0].ProjectSFID
		info.name = mappings[0].ProjectName
	}

	if projectSFID != "" && s.projectService != nil {
		f := logrus.Fields{
			"functionName":   "v2.my_clas.service.projectInfo",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     claGroupID,
			"projectSFID":    projectSFID,
		}
		project, projectErr := s.projectService.GetProject(projectSFID)
		if projectErr != nil {
			log.WithFields(f).WithError(projectErr).Warn("unable to load the project details for the CLA group - leaving the logo empty")
		} else if project != nil {
			if project.Name != "" {
				info.name = project.Name
			}
			info.logo = project.ProjectLogo
		}
	}

	cache[claGroupID] = info
	return info, nil
}

// foundationMapping returns the mapping row that marks a foundation-level CLA Group
// (ProjectSFID == FoundationSFID, the projects_cla_groups convention used by
// SignedAtFoundation), or nil when the CLA Group is not foundation-level.
func foundationMapping(mappings []*projects_cla_groups.ProjectClaGroup) *projects_cla_groups.ProjectClaGroup {
	for _, m := range mappings {
		if m.FoundationSFID != "" && m.FoundationSFID == m.ProjectSFID {
			return m
		}
	}
	return nil
}

func (s *service) company(ctx context.Context, cache map[string]*v1Models.Company, companyID string) (*v1Models.Company, error) {
	if companyModel, ok := cache[companyID]; ok {
		return companyModel, nil
	}
	companyModel, err := s.companyRepo.GetCompany(ctx, companyID)
	if err != nil {
		var companyNotFound *utils.CompanyNotFound
		if !errors.As(err, &companyNotFound) {
			cache[companyID] = nil
			return nil, err
		}
		companyModel = nil
	}
	cache[companyID] = companyModel
	return companyModel, nil
}

// signedOn mirrors the v1 signatures converter: prefer signed_on, fall back to date_created
func signedOn(sig *signatures.ItemSignature) string {
	value := sig.DateCreated
	if sig.SignedOn != "" {
		value = sig.SignedOn
	}
	if value != "" {
		value = utils.FormatTimeString(value)
	}
	return value
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func normalizeEmails(emails []string) []string {
	normalized := make([]string, 0, len(emails))
	seen := make(map[string]bool)
	for _, email := range emails {
		email = normalizeEmail(email)
		if email == "" || seen[email] {
			continue
		}
		seen[email] = true
		normalized = append(normalized, email)
	}
	return normalized
}

func trimAll(values []string) []string {
	trimmed := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" && !seen[value] {
			seen[value] = true
			trimmed = append(trimmed, value)
		}
	}
	return trimmed
}

func dedupeIDs(ids []int64) []int64 {
	deduped := make([]int64, 0, len(ids))
	seen := make(map[int64]bool)
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			deduped = append(deduped, id)
		}
	}
	return deduped
}
