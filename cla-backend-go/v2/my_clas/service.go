// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	openapiErrors "github.com/go-openapi/errors"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	platformModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
	"github.com/sirupsen/logrus"
)

// Identity sources reported by the platform user-service identities API
const (
	identitySourceGithub = "github"
	identitySourceGitlab = "gitlab"
	identitySourceGerrit = "gerrit"
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

// IsEmpty returns true when no identity key was provided
func (i *Identity) IsEmpty() bool {
	return i.LfUsername == "" && len(i.Emails) == 0 && len(i.SecondaryEmails) == 0 &&
		len(i.GithubIDs) == 0 && len(i.GithubUsernames) == 0 &&
		len(i.GitlabIDs) == 0 && len(i.GitlabUsernames) == 0 &&
		len(i.GerritUsernames) == 0
}

// UsersService is the subset of the users service used to resolve identities to user records
type UsersService interface {
	GetUserByLFUserName(lfUserName string) (*v1Models.User, error)
	GetUsersByLFEmail(userEmail string) ([]*v1Models.User, error)
	GetUsersByEmail(userEmail string) ([]*v1Models.User, error)
	GetUserByGitHubID(gitHubID string) (*v1Models.User, error)
	GetUserByGitHubUsername(gitHubUsername string) (*v1Models.User, error)
	GetUserByGitlabID(gitLabID int) (*v1Models.User, error)
	GetUserByGitLabUsername(gitLabUsername string) (*v1Models.User, error)
}

// PlatformUsersService is the subset of the platform user-service client used to verify
// that identities are connected to the authenticated user's LF account
type PlatformUsersService interface {
	GetUserByUsername(lfUsername string) (*platformModels.User, error)
	ListUserIdentities(userSFID string) ([]*platformModels.UserIdentity, error)
}

// SignaturesService is the subset of the v1 signatures service used to evaluate ECLA validity
type SignaturesService interface {
	GetCorporateSignature(ctx context.Context, claGroupID, companyID string, approved, signed *bool) (*v1Models.Signature, error)
	UserIsApproved(ctx context.Context, user *v1Models.User, cclaSignature *v1Models.Signature) (bool, error)
}

// CompanyRepository is the subset of the company repository used to resolve employers
type CompanyRepository interface {
	GetCompany(ctx context.Context, companyID string) (*v1Models.Company, error)
}

// ProjectsCLAGroupsRepository is the subset of the projects-cla-groups repository used to resolve CLA Group names
type ProjectsCLAGroupsRepository interface {
	GetCLAGroupNameByID(ctx context.Context, claGroupID string) (string, error)
}

// Service interface defines the My CLAs service methods
type Service interface {
	GetMyClas(ctx context.Context, currentUsername string, admin bool, requested *Identity) (*models.MyClaList, error)
	GetMyClaPdfURL(ctx context.Context, currentUsername string, admin bool, requested *Identity, signatureID string) (*models.MyClaPdf, error)
}

type service struct {
	repo                  Repository
	usersService          UsersService
	platformUsersService  PlatformUsersService
	signaturesService     SignaturesService
	companyRepo           CompanyRepository
	projectsClaGroupsRepo ProjectsCLAGroupsRepository
	presign               func(filename string) (string, error)
}

// NewService creates a new instance of the My CLAs service
func NewService(repo Repository, usersService UsersService, platformUsersService PlatformUsersService, signaturesService SignaturesService, companyRepo CompanyRepository, projectsClaGroupsRepo ProjectsCLAGroupsRepository) Service {
	return &service{
		repo:                  repo,
		usersService:          usersService,
		platformUsersService:  platformUsersService,
		signaturesService:     signaturesService,
		companyRepo:           companyRepo,
		projectsClaGroupsRepo: projectsClaGroupsRepo,
		presign:               utils.GetDownloadLink,
	}
}

// GetMyClas returns all signed ICLAs and ECLAs of the EasyCLA user records matching the
// given identity, with validity evaluated against the current CCLA approval lists. For
// non-admin callers every requested identity key is verified to belong to the
// authenticated user (currentUsername) first; unverifiable keys are skipped and reported.
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
	companies := make(map[string]*v1Models.Company)
	cclas := make(map[string]*v1Models.Signature)
	approvals := make(map[string]bool)

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

			row := models.MyCla{
				SignatureID:          sig.SignatureID,
				ClaGroupID:           sig.SignatureProjectID,
				ClaGroupName:         s.claGroupName(ctx, claGroupNames, sig.SignatureProjectID),
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
			} else {
				row.ClaType = utils.ClaTypeECLA
				row.CompanyID = sig.SignatureUserCompanyID
				companyModel := s.company(ctx, companies, sig.SignatureUserCompanyID)
				if companyModel != nil {
					row.CompanyName = companyModel.CompanyName
					row.SigningEntityName = companyModel.SigningEntityName
				}
				covered, coveredErr := s.eclaCoveredByCurrentApprovalList(ctx, cclas, approvals, userModel, companyModel, sig)
				if coveredErr != nil {
					return nil, coveredErr
				}
				row.Valid = sig.SignatureApproved && covered
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
// signature belongs to one of the EasyCLA user records matching the given identity.
// The same identity-ownership enforcement as GetMyClas applies, so a non-admin caller
// can only ever resolve their own signatures. A nil result means not found - unknown,
// not-owned, unsigned or ECLA signature ID.
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

// effectiveIdentity returns the identity that is actually searched. Admin callers may
// search any identity (the LF username defaults to their own when not provided);
// everyone else is restricted to identities verified to belong to currentUsername.
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

// platformIdentitySet holds the identities connected to the LF account per the platform
// user-service - emails across all sources plus usernames per source (github/gitlab/gerrit)
type platformIdentitySet struct {
	emails    map[string]bool
	usernames map[string]map[string]bool
}

// authorizeIdentity verifies each requested identity key against the authenticated
// user's own EasyCLA user record first and, when not covered there, against the
// identities connected to their LF account in the platform user-service. Keys that
// cannot be verified are dropped from the search and reported back.
func (s *service) authorizeIdentity(ctx context.Context, currentUsername string, requested *Identity) (*Identity, []string, error) {
	f := logrus.Fields{
		"functionName":    "v2.my_clas.service.authorizeIdentity",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"currentUsername": currentUsername,
	}

	allowed := &Identity{LfUsername: currentUsername}
	skipped := []string{}

	selfUser, err := s.usersService.GetUserByLFUserName(currentUsername)
	if err != nil && !isNotFound(err) {
		log.WithFields(f).WithError(err).Warn("unable to lookup the authenticated user's EasyCLA record")
		return nil, nil, err
	}

	selfEmails := make(map[string]bool)
	var selfGithubID, selfGithubUsername, selfGitlabID, selfGitlabUsername string
	if selfUser != nil {
		if selfUser.LfEmail != "" {
			selfEmails[strings.ToLower(strings.TrimSpace(string(selfUser.LfEmail)))] = true
		}
		for _, email := range selfUser.Emails {
			if email != "" {
				selfEmails[strings.ToLower(strings.TrimSpace(email))] = true
			}
		}
		selfGithubID = strings.TrimSpace(selfUser.GithubID)
		selfGithubUsername = strings.TrimSpace(selfUser.GithubUsername)
		selfGitlabID = strings.TrimSpace(selfUser.GitlabID)
		selfGitlabUsername = strings.TrimSpace(selfUser.GitlabUsername)
	}

	// LF-wide identities are loaded lazily - only when a requested key is not already
	// covered by the EasyCLA user record
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
	usernameAllowed := func(source, username string) bool {
		return platformIdentities().usernames[source][strings.ToLower(username)]
	}

	if requested.LfUsername != "" && !strings.EqualFold(requested.LfUsername, currentUsername) {
		skipped = append(skipped, "lfUsername:"+requested.LfUsername)
	}

	for _, email := range requested.Emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}
		if emailAllowed(email) {
			allowed.Emails = append(allowed.Emails, email)
		} else {
			skipped = append(skipped, "email:"+email)
		}
	}

	for _, email := range requested.SecondaryEmails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}
		if emailAllowed(email) {
			allowed.SecondaryEmails = append(allowed.SecondaryEmails, email)
		} else {
			skipped = append(skipped, "secondaryEmail:"+email)
		}
	}

	for _, githubID := range requested.GithubIDs {
		if selfGithubID != "" && strconv.FormatInt(githubID, 10) == selfGithubID {
			allowed.GithubIDs = append(allowed.GithubIDs, githubID)
		} else {
			skipped = append(skipped, fmt.Sprintf("githubId:%d", githubID))
		}
	}

	for _, githubUsername := range requested.GithubUsernames {
		githubUsername = strings.TrimSpace(githubUsername)
		if githubUsername == "" {
			continue
		}
		if (selfGithubUsername != "" && strings.EqualFold(githubUsername, selfGithubUsername)) || usernameAllowed(identitySourceGithub, githubUsername) {
			allowed.GithubUsernames = append(allowed.GithubUsernames, githubUsername)
		} else {
			skipped = append(skipped, "githubUsername:"+githubUsername)
		}
	}

	for _, gitlabID := range requested.GitlabIDs {
		if selfGitlabID != "" && strconv.FormatInt(gitlabID, 10) == selfGitlabID {
			allowed.GitlabIDs = append(allowed.GitlabIDs, gitlabID)
		} else {
			skipped = append(skipped, fmt.Sprintf("gitlabId:%d", gitlabID))
		}
	}

	for _, gitlabUsername := range requested.GitlabUsernames {
		gitlabUsername = strings.TrimSpace(gitlabUsername)
		if gitlabUsername == "" {
			continue
		}
		if (selfGitlabUsername != "" && strings.EqualFold(gitlabUsername, selfGitlabUsername)) || usernameAllowed(identitySourceGitlab, gitlabUsername) {
			allowed.GitlabUsernames = append(allowed.GitlabUsernames, gitlabUsername)
		} else {
			skipped = append(skipped, "gitlabUsername:"+gitlabUsername)
		}
	}

	for _, gerritUsername := range requested.GerritUsernames {
		gerritUsername = strings.TrimSpace(gerritUsername)
		if gerritUsername == "" {
			continue
		}
		if strings.EqualFold(gerritUsername, currentUsername) || usernameAllowed(identitySourceGerrit, gerritUsername) {
			allowed.GerritUsernames = append(allowed.GerritUsernames, gerritUsername)
		} else {
			skipped = append(skipped, "gerritUsername:"+gerritUsername)
		}
	}

	return allowed, skipped, nil
}

// loadPlatformIdentities collects the emails and per-source usernames connected to the
// LF account per the platform user-service. Lookup failures are logged and yield an
// empty set - the affected identity keys are then skipped (and reported), never allowed.
func (s *service) loadPlatformIdentities(ctx context.Context, lfUsername string) *platformIdentitySet {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.service.loadPlatformIdentities",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"lfUsername":     lfUsername,
	}

	set := &platformIdentitySet{
		emails: make(map[string]bool),
		usernames: map[string]map[string]bool{
			identitySourceGithub: make(map[string]bool),
			identitySourceGitlab: make(map[string]bool),
			identitySourceGerrit: make(map[string]bool),
		},
	}

	if s.platformUsersService == nil {
		return set
	}

	platformUser, err := s.platformUsersService.GetUserByUsername(lfUsername)
	if err != nil || platformUser == nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup the LF user in the platform user-service")
		return set
	}

	if platformUser.Email != nil && *platformUser.Email != "" {
		set.emails[strings.ToLower(strings.TrimSpace(*platformUser.Email))] = true
	}
	for _, email := range platformUser.Emails {
		if email == nil || email.EmailAddress == nil || *email.EmailAddress == "" {
			continue
		}
		if email.IsDeleted != nil && *email.IsDeleted {
			continue
		}
		set.emails[strings.ToLower(strings.TrimSpace(*email.EmailAddress))] = true
	}

	if platformUser.ID == "" {
		return set
	}
	identityList, err := s.platformUsersService.ListUserIdentities(platformUser.ID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to list the LF user's connected identities")
		return set
	}
	for _, identity := range identityList {
		if identity == nil {
			continue
		}
		source := strings.ToLower(strings.TrimSpace(identity.Source))
		if identity.Username != "" {
			if _, ok := set.usernames[source]; ok {
				set.usernames[source][strings.ToLower(strings.TrimSpace(identity.Username))] = true
			}
		}
		if identity.Email != "" {
			set.emails[strings.ToLower(strings.TrimSpace(identity.Email))] = true
		}
	}

	return set
}

// resolveUsers unions the EasyCLA user records matching any of the identity keys,
// deduplicated by user ID. All lookups are backed by DynamoDB GSI queries except the
// explicitly opt-in secondary-email match, which requires a table scan (the user_emails
// attribute is a string set and cannot be indexed).
func (s *service) resolveUsers(ctx context.Context, identity *Identity) ([]*v1Models.User, error) {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.service.resolveUsers",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	var userModels []*v1Models.User
	seen := make(map[string]bool)
	add := func(userModel *v1Models.User) {
		if userModel == nil || userModel.UserID == "" || seen[userModel.UserID] {
			return
		}
		seen[userModel.UserID] = true
		userModels = append(userModels, userModel)
	}

	lfUsernames := identity.GerritUsernames
	if identity.LfUsername != "" {
		lfUsernames = append([]string{identity.LfUsername}, lfUsernames...)
	}
	for _, lfUsername := range lfUsernames {
		lfUsername = strings.TrimSpace(lfUsername)
		if lfUsername == "" {
			continue
		}
		userModel, err := s.usersService.GetUserByLFUserName(lfUsername)
		if err != nil && !isNotFound(err) {
			log.WithFields(f).WithError(err).Warnf("unable to lookup user by LF username: %s", lfUsername)
			return nil, err
		}
		add(userModel)
	}

	for _, email := range identity.Emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}
		matches, err := s.usersService.GetUsersByLFEmail(email)
		if err != nil && !isNotFound(err) {
			log.WithFields(f).WithError(err).Warnf("unable to lookup users by email: %s", email)
			return nil, err
		}
		for _, userModel := range matches {
			add(userModel)
		}
	}

	for _, email := range identity.SecondaryEmails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}
		matches, err := s.usersService.GetUsersByEmail(email)
		if err != nil && !isNotFound(err) {
			log.WithFields(f).WithError(err).Warnf("unable to lookup users by secondary email: %s", email)
			return nil, err
		}
		for _, userModel := range matches {
			add(userModel)
		}
	}

	for _, githubID := range identity.GithubIDs {
		userModel, err := s.usersService.GetUserByGitHubID(strconv.FormatInt(githubID, 10))
		if err != nil && !isNotFound(err) {
			log.WithFields(f).WithError(err).Warnf("unable to lookup user by GitHub ID: %d", githubID)
			return nil, err
		}
		add(userModel)
	}

	for _, githubUsername := range identity.GithubUsernames {
		githubUsername = strings.TrimSpace(githubUsername)
		if githubUsername == "" {
			continue
		}
		userModel, err := s.usersService.GetUserByGitHubUsername(githubUsername)
		if err != nil && !isNotFound(err) {
			log.WithFields(f).WithError(err).Warnf("unable to lookup user by GitHub username: %s", githubUsername)
			return nil, err
		}
		add(userModel)
	}

	for _, gitlabID := range identity.GitlabIDs {
		userModel, err := s.usersService.GetUserByGitlabID(int(gitlabID))
		if err != nil && !isNotFound(err) {
			log.WithFields(f).WithError(err).Warnf("unable to lookup user by GitLab ID: %d", gitlabID)
			return nil, err
		}
		add(userModel)
	}

	for _, gitlabUsername := range identity.GitlabUsernames {
		gitlabUsername = strings.TrimSpace(gitlabUsername)
		if gitlabUsername == "" {
			continue
		}
		userModel, err := s.usersService.GetUserByGitLabUsername(gitlabUsername)
		if err != nil && !isNotFound(err) {
			log.WithFields(f).WithError(err).Warnf("unable to lookup user by GitLab username: %s", gitlabUsername)
			return nil, err
		}
		add(userModel)
	}

	return userModels, nil
}

// eclaCoveredByCurrentApprovalList evaluates whether an ECLA is still covered by the
// employer's current CCLA and its approval lists, mirroring the PR gating logic
// (signatures service ProcessEmployeeSignature/UserIsApproved): the company must not be
// sanctioned, the company must hold an approved+signed CCLA for the CLA Group, and the
// user must match the current approval lists.
func (s *service) eclaCoveredByCurrentApprovalList(ctx context.Context, cclas map[string]*v1Models.Signature, approvals map[string]bool, userModel *v1Models.User, companyModel *v1Models.Company, sig *signatures.ItemSignature) (bool, error) {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.service.eclaCoveredByCurrentApprovalList",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    sig.SignatureID,
		"claGroupID":     sig.SignatureProjectID,
		"companyID":      sig.SignatureUserCompanyID,
	}

	if companyModel == nil || companyModel.IsSanctioned {
		return false, nil
	}

	cclaKey := sig.SignatureProjectID + "|" + sig.SignatureUserCompanyID
	ccla, ok := cclas[cclaKey]
	if !ok {
		approved, signed := true, true
		cclaModel, cclaErr := s.signaturesService.GetCorporateSignature(ctx, sig.SignatureProjectID, sig.SignatureUserCompanyID, &approved, &signed)
		if cclaErr != nil {
			log.WithFields(f).WithError(cclaErr).Warn("unable to lookup the corporate signature for the employee acknowledgement")
			return false, cclaErr
		}
		ccla = cclaModel
		cclas[cclaKey] = ccla
	}
	if ccla == nil {
		return false, nil
	}

	approvalKey := cclaKey + "|" + userModel.UserID
	if covered, ok := approvals[approvalKey]; ok {
		return covered, nil
	}
	covered, approvedErr := s.signaturesService.UserIsApproved(ctx, userModel, ccla)
	if approvedErr != nil {
		// Mirror the gating behavior for approval-list evaluation problems (e.g. a
		// malformed domain pattern): log and treat as not covered rather than failing
		// the whole listing.
		log.WithFields(f).WithError(approvedErr).Warn("unable to evaluate the approval list for the employee acknowledgement")
		covered = false
	}
	approvals[approvalKey] = covered
	return covered, nil
}

// claGroupName resolves and caches the CLA Group name, returning an empty string when
// the CLA Group record cannot be resolved.
func (s *service) claGroupName(ctx context.Context, cache map[string]string, claGroupID string) string {
	if claGroupID == "" {
		return ""
	}
	if name, ok := cache[claGroupID]; ok {
		return name
	}
	name, err := s.projectsClaGroupsRepo.GetCLAGroupNameByID(ctx, claGroupID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"functionName":   "v2.my_clas.service.claGroupName",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     claGroupID,
		}).WithError(err).Warn("unable to lookup the CLA Group name")
		name = ""
	}
	cache[claGroupID] = name
	return name
}

// company resolves and caches the company record, returning nil when it cannot be resolved.
func (s *service) company(ctx context.Context, cache map[string]*v1Models.Company, companyID string) *v1Models.Company {
	if companyModel, ok := cache[companyID]; ok {
		return companyModel
	}
	companyModel, err := s.companyRepo.GetCompany(ctx, companyID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"functionName":   "v2.my_clas.service.company",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"companyID":      companyID,
		}).WithError(err).Warn("unable to lookup the company record")
		companyModel = nil
	}
	cache[companyID] = companyModel
	return companyModel
}

// signedOn mirrors the v1 signatures converter behavior: prefer the signed_on value and
// fall back to date_created for older records missing it.
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

// isNotFound returns true when the given lookup error only indicates that no user
// record matched the identity key.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var userNotFound *utils.UserNotFound
	if errors.As(err, &userNotFound) {
		return true
	}
	var apiErr openapiErrors.Error
	if errors.As(err, &apiErr) && apiErr.Code() == http.StatusNotFound {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "not found")
}
