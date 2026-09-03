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

	"github.com/gofrs/uuid"
	"github.com/linuxfoundation/easycla/cla-backend-go/emails"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	v2ProjectServiceModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service/models"
	platformModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	identitySummaryLimit       = 512
	identitySourceGithub       = "github"
	identitySourceGitlab       = "gitlab"
	identitySourceGerrit       = "gerrit"
	identityDataSourcePlatform = "platform"
)

// Identity holds the caller-provided identity keys used to resolve EasyCLA user records.
// Caller-supplied keys are transitional (P3/P9 of the trust-SS decision): at M6 EasyCLA should
// call lfx.auth-service.user_identity.list over NATS itself and drop them with the azp allow-list.
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

// Summary renders the identity keys, length-bounded, for the caller audit log
func (i *Identity) Summary() string {
	var parts []string
	addStrings := func(param string, values []string) {
		if trimmed := trimAll(values); len(trimmed) > 0 {
			parts = append(parts, param+":"+strings.Join(trimmed, ","))
		}
	}
	addIDs := func(param string, values []int64) {
		ids := dedupeIDs(values)
		if len(ids) == 0 {
			return
		}
		formatted := make([]string, 0, len(ids))
		for _, id := range ids {
			formatted = append(formatted, strconv.FormatInt(id, 10))
		}
		parts = append(parts, param+":"+strings.Join(formatted, ","))
	}

	addStrings("lfUsername", []string{i.LfUsername})
	addStrings("email", i.Emails)
	addStrings("secondaryEmail", i.SecondaryEmails)
	addIDs("githubId", i.GithubIDs)
	addStrings("githubUsername", i.GithubUsernames)
	addIDs("gitlabId", i.GitlabIDs)
	addStrings("gitlabUsername", i.GitlabUsernames)
	addStrings("gerritUsername", i.GerritUsernames)

	summary := strings.Join(parts, " ")
	if len(summary) > identitySummaryLimit {
		summary = strings.ToValidUTF8(summary[:identitySummaryLimit], "") + "..."
	}
	return summary
}

// Caller is the authenticated principal a My CLAs lookup runs as
type Caller struct {
	Username string
	Admin    bool
	Trusted  bool
}

func hasValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

// PlatformUsersService is the user-service subset used to verify that identities are connected
// to the authenticated user's LF account
type PlatformUsersService interface {
	GetUserByUsernameContext(ctx context.Context, lfUsername string) (*platformModels.User, error)
	ListUserIdentities(ctx context.Context, userSFID string) ([]*platformModels.UserIdentity, error)
}

// SignaturesService is the v1 signatures subset used to evaluate ECLA validity
type SignaturesService interface {
	GetCorporateSignature(ctx context.Context, claGroupID, companyID string, approved, signed *bool) (*v1Models.Signature, error)
	EvaluateUserApproval(ctx context.Context, user *v1Models.User, cclaSignature *v1Models.Signature) (approved bool, githubOrgLookupFailed bool, err error)
}

// CompanyRepository is the company repository subset used to resolve employers
type CompanyRepository interface {
	GetCompany(ctx context.Context, companyID string) (*v1Models.Company, error)
	UpdateCompanySanctionStatus(ctx context.Context, companyID string, sanctioned bool, origin string) error
}

// ProjectsCLAGroupsRepository is the projects-cla-groups subset used to resolve CLA Group names
// and their Salesforce project mappings
type ProjectsCLAGroupsRepository interface {
	GetCLAGroupNameByID(ctx context.Context, claGroupID string) (string, error)
	GetProjectsIdsForClaGroup(ctx context.Context, claGroupID string) ([]*projects_cla_groups.ProjectClaGroup, error)
}

// ProjectService is the project-service subset used to resolve a project's display name and logo
type ProjectService interface {
	GetProject(projectSFID string) (*v2ProjectServiceModels.ProjectOutputDetailed, error)
}

// EventsService is the v1 events subset used to audit contact-CLA-manager requests
type EventsService interface {
	LogEventWithContext(ctx context.Context, args *events.LogEventArgs)
}

// ErrInvalidRecipients is returned when recipients is not a non-empty subset of the resolved CLA
// managers - empty is valid only when none resolves
var ErrInvalidRecipients = errors.New("recipients must be a non-empty subset of the CLA managers returned by the cla-managers endpoint - empty only when no CLA manager resolves")

// ErrMissingMessage is returned when a contact request carries no (non-blank) message
var ErrMissingMessage = errors.New("message is required for a contact request and must not be blank")

// Service interface defines the My CLAs service methods
type Service interface {
	GetMyClas(ctx context.Context, caller *Caller, requested *Identity) (*models.MyClaList, error)
	GetMyClaPdfURL(ctx context.Context, caller *Caller, requested *Identity, signatureID string) (*models.MyClaPdf, error)
	GetMyIdentities(ctx context.Context, currentUsername string) (*models.MyIdentityList, error)
	AuthorizeIdentity(ctx context.Context, currentUsername string, admin bool, requested *Identity) (*Identity, []string, error)
	GetMyClaManagers(ctx context.Context, caller *Caller, requested *Identity, signatureID string) (*models.MyClaManagerList, error)
	CreateMyClaManagerRequest(ctx context.Context, caller *Caller, requested *Identity, signatureID string, input *models.MyClaManagerRequest) (*models.MyClaManagerRequestResult, error)
}

type service struct {
	repo                  Repository
	platformUsersService  PlatformUsersService
	auth0Identities       Auth0IdentityService
	signaturesService     SignaturesService
	companyRepo           CompanyRepository
	projectsClaGroupsRepo ProjectsCLAGroupsRepository
	projectService        ProjectService
	eventsService         EventsService
	sanctions             SanctionsScreener
	presign               func(filename string) (string, error)
	documentExists        func(filename string) (bool, error)
	sendEmail             func(subject string, body string, recipients []string) error
}

// NewService creates a new instance of the My CLAs service
func NewService(repo Repository, platformUsersService PlatformUsersService, auth0Identities Auth0IdentityService, signaturesService SignaturesService, companyRepo CompanyRepository, projectsClaGroupsRepo ProjectsCLAGroupsRepository, projectService ProjectService, eventsService EventsService, sanctions SanctionsScreener) Service {
	return &service{
		repo:                  repo,
		platformUsersService:  platformUsersService,
		auth0Identities:       auth0Identities,
		signaturesService:     signaturesService,
		companyRepo:           companyRepo,
		projectsClaGroupsRepo: projectsClaGroupsRepo,
		projectService:        projectService,
		eventsService:         eventsService,
		sanctions:             sanctions,
		presign:               utils.GetDownloadLink,
		documentExists:        utils.DocumentExists,
		sendEmail:             utils.SendEmail,
	}
}

// projectInfo is the resolved Salesforce project display name, logo, and ids of a CLA Group
type projectInfo struct {
	name           string
	logo           string
	projectSFID    string
	foundationSFID string
}

// GetMyClas returns the signed ICLAs and ECLAs of every EasyCLA user record matching the identity,
// with validity evaluated against the current CCLA approval lists
func (s *service) GetMyClas(ctx context.Context, caller *Caller, requested *Identity) (*models.MyClaList, error) {
	f := logrus.Fields{
		"functionName":    "v2.my_clas.service.GetMyClas",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"currentUsername": callerUsername(caller),
		"admin":           caller != nil && caller.Admin,
		"trustedCaller":   caller != nil && caller.Trusted,
	}

	identity, skipped, err := s.effectiveIdentity(ctx, caller, requested)
	if err != nil {
		return nil, err
	}

	userModels, err := s.resolveUsers(ctx, identity)
	if err != nil {
		return nil, err
	}

	perUser, err := s.userSignatures(ctx, userModels)
	if err != nil {
		return nil, err
	}
	refs := claRefs(userModels, perUser)
	data, err := s.prefetch(ctx, refs)
	if err != nil {
		return nil, err
	}

	result := &models.MyClaList{
		LfUsername:        identity.LfUsername,
		UserIds:           make([]string, 0, len(userModels)),
		SkippedIdentities: skipped,
		Clas:              make([]models.MyCla, 0, len(refs)),
		SssMode:           s.sanctionsMode(),
	}
	for _, userModel := range userModels {
		result.UserIds = append(result.UserIds, userModel.UserID)
	}

	for _, ref := range refs {
		sig := ref.sig
		project := data.projectInfos[sig.SignatureProjectID]
		row := models.MyCla{
			SignatureID:          sig.SignatureID,
			ClaGroupID:           sig.SignatureProjectID,
			ClaGroupName:         data.claGroupNames[sig.SignatureProjectID],
			ProjectName:          project.name,
			ProjectLogo:          project.logo,
			ProjectSFID:          project.projectSFID,
			FoundationSFID:       project.foundationSFID,
			UserID:               sig.SignatureReferenceID,
			SignedOn:             signedOn(sig),
			Signed:               sig.SignatureSigned,
			Approved:             sig.SignatureApproved,
			DocumentMajorVersion: int64(sig.SignatureDocumentMajorVersion),
			DocumentMinorVersion: int64(sig.SignatureDocumentMinorVersion),
		}
		row.SignedVia, row.SignedAs = resolveSignedIdentity(sig, ref.user)
		if sig.DateInvalidated != "" {
			row.InvalidatedAt = utils.FormatTimeString(sig.DateInvalidated)
		}

		if sig.SignatureUserCompanyID == "" {
			row.ClaType = utils.ClaTypeICLA
			row.Valid = sig.SignatureApproved
			row.PdfAvailable = true
			assignMyClaStatus(&row, eclaCoverage{})
		} else {
			row.ClaType = utils.ClaTypeECLA
			row.CompanyID = sig.SignatureUserCompanyID
			if companyModel := data.companies[sig.SignatureUserCompanyID]; companyModel != nil {
				row.CompanyName = companyModel.CompanyName
				row.SigningEntityName = companyModel.SigningEntityName
			}
			sanction := data.sanctions[sig.SignatureUserCompanyID]
			row.Flagged = sanction.flagged
			row.FlaggedCheck = sanction.check
			if sanction.flagged && sanction.date != "" {
				row.FlaggedAt = utils.FormatTimeString(sanction.date)
			}
			coverage := data.coverage(sig, ref.user, sanction.flagged)
			row.Valid = sig.SignatureApproved && coverage.covered
			row.ClaManager = data.claManager(sig, identity.LfUsername, sanction.flagged)
			assignMyClaStatus(&row, coverage)
		}

		result.Clas = append(result.Clas, row)
	}

	sort.SliceStable(result.Clas, func(i, j int) bool {
		return result.Clas[i].SignedOn > result.Clas[j].SignedOn
	})
	result.ResultCount = int64(len(result.Clas))

	log.WithFields(f).Debugf("resolved %d user records with %d CLA records (%d identity keys skipped)", len(result.UserIds), result.ResultCount, len(skipped))
	return result, nil
}

// GetMyClaPdfURL returns a time-limited download URL for a signed ICLA PDF owned by the identity -
// nil means unknown, not-owned, unsigned or ECLA signature ID
func (s *service) GetMyClaPdfURL(ctx context.Context, caller *Caller, requested *Identity, signatureID string) (*models.MyClaPdf, error) {
	f := logrus.Fields{
		"functionName":    "v2.my_clas.service.GetMyClaPdfURL",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"currentUsername": callerUsername(caller),
		"admin":           caller != nil && caller.Admin,
		"trustedCaller":   caller != nil && caller.Trusted,
		"signatureID":     signatureID,
	}

	identity, _, err := s.effectiveIdentity(ctx, caller, requested)
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

// GetMyClaManagers returns the CLA managers of the CCLA covering the given ECLA - nil means
// unknown, not-owned, unsigned or ICLA signature ID
func (s *service) GetMyClaManagers(ctx context.Context, caller *Caller, requested *Identity, signatureID string) (*models.MyClaManagerList, error) {
	identity, sig, _, err := s.findOwnedEcla(ctx, caller, requested, signatureID)
	if err != nil || sig == nil {
		return nil, err
	}

	details, err := s.eclaManagerDetails(ctx, identity, sig)
	if err != nil {
		return nil, err
	}

	return &models.MyClaManagerList{
		SignatureID:  sig.SignatureID,
		ClaGroupID:   sig.SignatureProjectID,
		ClaGroupName: details.claGroupName,
		ProjectName:  details.projectName,
		CompanyID:    sig.SignatureUserCompanyID,
		CompanyName:  details.companyName,
		ClaManager:   details.callerIsManager,
		Managers:     details.managers,
		ResultCount:  int64(len(details.managers)),
	}, nil
}

// CreateMyClaManagerRequest emails a removal/approval/contact request against the caller's own
// ECLA to the selected CLA managers and logs the audit event that is its receipt - nil means
// unknown, not-owned, unsigned or ICLA signature ID, ErrInvalidRecipients an invalid recipients
// list, ErrMissingMessage a contact request without a message
func (s *service) CreateMyClaManagerRequest(ctx context.Context, caller *Caller, requested *Identity, signatureID string, input *models.MyClaManagerRequest) (*models.MyClaManagerRequestResult, error) {
	f := logrus.Fields{
		"functionName":    "v2.my_clas.service.CreateMyClaManagerRequest",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"currentUsername": callerUsername(caller),
		"signatureID":     signatureID,
	}

	identity, sig, userModel, err := s.findOwnedEcla(ctx, caller, requested, signatureID)
	if err != nil || sig == nil {
		return nil, err
	}

	details, err := s.eclaManagerDetails(ctx, identity, sig)
	if err != nil {
		return nil, err
	}

	byUsername := make(map[string]models.MyClaManager, len(details.managers))
	for _, manager := range details.managers {
		byUsername[strings.ToLower(manager.LfUsername)] = manager
	}
	recipients := trimAll(input.Recipients)
	if len(details.managers) > 0 && len(recipients) == 0 {
		return nil, ErrInvalidRecipients
	}
	selectedUsernames := make([]string, 0, len(recipients))
	recipientEmails := make([]string, 0, len(recipients))
	selected := make(map[string]bool, len(recipients))
	emailed := make(map[string]bool, len(recipients))
	for _, recipient := range recipients {
		key := strings.ToLower(recipient)
		manager, ok := byUsername[key]
		if !ok {
			return nil, ErrInvalidRecipients
		}
		if selected[key] {
			continue
		}
		selected[key] = true
		selectedUsernames = append(selectedUsernames, manager.LfUsername)
		// Managers can share an address; mail it once, but report both as recipients.
		if emailKey := strings.ToLower(manager.Email); emailKey != "" && !emailed[emailKey] {
			emailed[emailKey] = true
			recipientEmails = append(recipientEmails, manager.Email)
		}
	}

	requestType := utils.StringValue(input.RequestType)
	message := utils.SanitizePlainText(input.Message)
	if requestType == models.MyClaManagerRequestRequestTypeContact && message == "" {
		return nil, ErrMissingMessage
	}
	contributorName := userModel.Username
	if contributorName == "" {
		contributorName = identity.LfUsername
	}
	_, contributorIdentity := resolveSignedIdentity(sig, userModel)
	if contributorIdentity == "" {
		contributorIdentity = identity.LfUsername
	}

	status := models.MyClaManagerRequestResultStatusRecorded
	body := ""
	if len(recipientEmails) > 0 {
		body, err = emails.RenderContactClaManagerTemplate(emails.ContactClaManagerTemplateParams{
			RequestAction:       requestAction(requestType),
			ContributorName:     contributorName,
			ContributorIdentity: contributorIdentity,
			ContributorEmail:    utils.GetBestEmail(userModel),
			CompanyName:         details.companyName,
			ProjectName:         details.projectName,
			CLAGroupName:        details.claGroupName,
			OptionalMessage:     message,
			ContactOnly:         requestType == models.MyClaManagerRequestRequestTypeContact,
		})
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to render the contact CLA manager email")
			return nil, err
		}
		status = models.MyClaManagerRequestResultStatusSent
	}

	requestUUID, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to generate a request ID")
		return nil, err
	}
	requestID := requestUUID.String()

	if len(recipientEmails) > 0 {
		subject := utils.SanitizeSingleLine(requestSubject(requestType, contributorName, details.companyName))
		if sendErr := s.sendEmail(subject, body, recipientEmails); sendErr != nil {
			log.WithFields(f).WithError(sendErr).Warn("unable to send the contact CLA manager email")
			return nil, sendErr
		}
	}

	if s.eventsService != nil {
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:    events.ContactCLAManagerRequestCreated,
			UserID:       userModel.UserID,
			LfUsername:   identity.LfUsername,
			UserName:     contributorName,
			CLAGroupID:   sig.SignatureProjectID,
			CLAGroupName: details.claGroupName,
			ProjectID:    sig.SignatureProjectID,
			ProjectName:  details.projectName,
			CompanyID:    sig.SignatureUserCompanyID,
			CompanyName:  details.companyName,
			EventData: &events.ContactCLAManagerRequestCreatedEventData{
				RequestID:   requestID,
				RequestType: requestType,
				SignatureID: sig.SignatureID,
				Message:     message,
				Recipients:  selectedUsernames,
			},
		})
	}

	return &models.MyClaManagerRequestResult{
		RequestID:   requestID,
		SignatureID: sig.SignatureID,
		RequestType: requestType,
		Status:      status,
		Recipients:  selectedUsernames,
	}, nil
}

// findOwnedEcla locates the signed ECLA with this ID among the identity's EasyCLA user records -
// the ownership boundary GetMyClaPdfURL enforces; nil means unknown, not-owned, unsigned or ICLA
func (s *service) findOwnedEcla(ctx context.Context, caller *Caller, requested *Identity, signatureID string) (*Identity, *signatures.ItemSignature, *v1Models.User, error) {
	identity, _, err := s.effectiveIdentity(ctx, caller, requested)
	if err != nil {
		return nil, nil, nil, err
	}

	userModels, err := s.resolveUsers(ctx, identity)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, userModel := range userModels {
		userSignatures, sigErr := s.repo.GetUserCLASignatures(ctx, userModel.UserID)
		if sigErr != nil {
			return nil, nil, nil, sigErr
		}
		for _, sig := range userSignatures {
			if sig.SignatureID != signatureID {
				continue
			}
			if !sig.SignatureSigned || sig.SignatureUserCompanyID == "" {
				return identity, nil, nil, nil
			}
			return identity, sig, userModel, nil
		}
	}

	return identity, nil, nil, nil
}

type managerDetails struct {
	claGroupName    string
	projectName     string
	companyName     string
	managers        []models.MyClaManager
	callerIsManager bool
}

// eclaManagerDetails resolves an ECLA's CLA Group/project/company context and the CLA managers
// from the covering CCLA's ACL - no CCLA yields an empty manager list
func (s *service) eclaManagerDetails(ctx context.Context, identity *Identity, sig *signatures.ItemSignature) (*managerDetails, error) {
	var (
		claGroupName string
		project      projectInfo
		companyModel *v1Models.Company
		ccla         *v1Models.Signature
	)
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		claGroupName, err = s.claGroupName(groupCtx, sig.SignatureProjectID)
		return err
	})
	group.Go(func() error {
		var err error
		project, err = s.projectInfo(groupCtx, sig.SignatureProjectID)
		return err
	})
	group.Go(func() error {
		var err error
		companyModel, err = s.company(groupCtx, sig.SignatureUserCompanyID)
		return err
	})
	group.Go(func() error {
		approved, signed := true, true
		var err error
		ccla, err = s.signaturesService.GetCorporateSignature(groupCtx, sig.SignatureProjectID, sig.SignatureUserCompanyID, &approved, &signed)
		return err
	})
	if err := group.Wait(); err != nil {
		return nil, err
	}

	details := &managerDetails{
		claGroupName: claGroupName,
		projectName:  project.name,
		managers:     []models.MyClaManager{},
	}
	if companyModel != nil {
		details.companyName = companyModel.CompanyName
	}
	if ccla == nil {
		return details, nil
	}

	for _, aclUser := range ccla.SignatureACL {
		lfUsername := aclUser.LfUsername
		if lfUsername == "" {
			lfUsername = aclUser.Username
		}
		if lfUsername == "" {
			continue
		}
		email := string(aclUser.LfEmail)
		if email == "" && len(aclUser.Emails) > 0 {
			email = aclUser.Emails[0]
		}
		details.managers = append(details.managers, models.MyClaManager{
			LfUsername: lfUsername,
			Name:       aclUser.Username,
			Email:      email,
		})
	}
	details.callerIsManager = isClaManager(ccla, identity.LfUsername)

	return details, nil
}

func requestAction(requestType string) string {
	if requestType == models.MyClaManagerRequestRequestTypeRemoval {
		return "removal from the corporate CLA coverage"
	}
	return "approval under the corporate CLA"
}

func requestSubject(requestType, contributorName, companyName string) string {
	if requestType == models.MyClaManagerRequestRequestTypeContact {
		return fmt.Sprintf("EasyCLA: Message from %s regarding %s", contributorName, companyName)
	}
	return fmt.Sprintf("EasyCLA: %s request from %s for %s", requestAction(requestType), contributorName, companyName)
}

func isClaManager(ccla *v1Models.Signature, lfUsername string) bool {
	if ccla == nil || lfUsername == "" {
		return false
	}
	for _, aclUser := range ccla.SignatureACL {
		if strings.EqualFold(aclUser.LfUsername, lfUsername) || strings.EqualFold(aclUser.Username, lfUsername) {
			return true
		}
	}
	return false
}

// resolveSignedIdentity derives the platform and account a signature was signed via/as. The
// platform hint - the platform-prefixed ACL entry first, else the recorded return URL type - is
// resolved against the signature's own identity attributes and then the owning user record,
// which fixes legacy rows whose asynchronous back-fill stamped only email/LF attributes (every
// pre-fix GitLab-signed row otherwise displays gerrit). Without a usable hint the fixed
// github > gitlab > gerrit precedence applies, over the row first and the user record last, so
// hint-less rows keep resolving exactly as before. Strictly read-only - nothing is written back
func resolveSignedIdentity(sig *signatures.ItemSignature, user *v1Models.User) (string, string) {
	rowGithub, rowGitlab, rowGerrit := rowIdentities(sig)
	userGithub, userGitlab, userGerrit := userIdentities(user)
	if hint := platformHint(sig); hint != "" {
		if via, as := hintedIdentity(hint, rowGithub, rowGitlab, rowGerrit); as != "" {
			return via, as
		}
		if via, as := hintedIdentity(hint, userGithub, userGitlab, userGerrit); as != "" {
			return via, as
		}
	}
	if via, as := precedenceIdentity(rowGithub, rowGitlab, rowGerrit); as != "" {
		return via, as
	}
	return precedenceIdentity(userGithub, userGitlab, userGerrit)
}

// platformHint returns the platform a signature was prepared under. A platform-prefixed ACL
// entry (github:<id> / gitlab:<id>, written by the PR/MR and Self Serve sign flows) is
// authoritative - a Self Serve session carries no pull or merge request, so the return URL type
// does not identify the signer there - with the recorded return URL type as the fallback for the
// PR/MR/Gerrit flows, where it is meaningful. Bare ACL entries are ambiguous (Gerrit LFIDs,
// LF-only fallbacks, legacy formats) and yield no hint, keeping such rows resolving as before.
// Known residue: the sign-URL regenerate flow rewrites both markers with the current flow, so a
// post-sign cross-platform regenerate can transiently shift the hint until the row is re-signed
func platformHint(sig *signatures.ItemSignature) string {
	for _, entry := range sig.SignatureACL {
		switch lower := strings.ToLower(strings.TrimSpace(entry)); {
		case strings.HasPrefix(lower, "github:"):
			return models.MyClaSignedViaGithub
		case strings.HasPrefix(lower, "gitlab:"):
			return models.MyClaSignedViaGitlab
		}
	}
	switch hint := strings.ToLower(sig.SignatureReturnURLType); hint {
	case models.MyClaSignedViaGithub, models.MyClaSignedViaGitlab, models.MyClaSignedViaGerrit:
		return hint
	}
	return ""
}

// rowIdentities extracts the per-platform signed-as candidates from the signature's own identity
// attributes
func rowIdentities(sig *signatures.ItemSignature) (githubAs, gitlabAs, gerritAs string) {
	githubAs = sig.UserGithubUsername
	if githubAs == "" {
		githubAs = sig.UserGithubID
	}
	gitlabAs = sig.UserGitlabUsername
	if gitlabAs == "" {
		gitlabAs = sig.UserGitlabID
	}
	gerritAs = sig.UserEmail
	if gerritAs == "" {
		gerritAs = sig.UserLFUsername
	}
	return githubAs, gitlabAs, gerritAs
}

// userIdentities extracts the per-platform signed-as candidates from the user record the
// signature belongs to - the display fallback for rows whose own identity attributes were never
// stamped at insert, were stamped partially by the legacy back-fill, or were dropped by legacy
// full-row sign-URL rewrites
func userIdentities(user *v1Models.User) (githubAs, gitlabAs, gerritAs string) {
	if user == nil {
		return "", "", ""
	}
	githubAs = user.GithubUsername
	if githubAs == "" {
		githubAs = user.GithubID
	}
	gitlabAs = user.GitlabUsername
	if gitlabAs == "" {
		gitlabAs = user.GitlabID
	}
	if user.LfEmail != "" {
		gerritAs = user.LfEmail.String()
	} else if user.LfUsername != "" {
		gerritAs = user.LfUsername
	}
	return githubAs, gitlabAs, gerritAs
}

// hintedIdentity returns the signed via/as pair for the hinted platform, or nothing when that
// platform carries no identity in the given candidates
func hintedIdentity(hint, githubAs, gitlabAs, gerritAs string) (string, string) {
	switch hint {
	case models.MyClaSignedViaGithub:
		if githubAs != "" {
			return models.MyClaSignedViaGithub, githubAs
		}
	case models.MyClaSignedViaGitlab:
		if gitlabAs != "" {
			return models.MyClaSignedViaGitlab, gitlabAs
		}
	case models.MyClaSignedViaGerrit:
		if gerritAs != "" {
			return models.MyClaSignedViaGerrit, gerritAs
		}
	}
	return "", ""
}

// precedenceIdentity resolves the signed via/as pair by the fixed github > gitlab > gerrit
// precedence
func precedenceIdentity(githubAs, gitlabAs, gerritAs string) (string, string) {
	switch {
	case githubAs != "":
		return models.MyClaSignedViaGithub, githubAs
	case gitlabAs != "":
		return models.MyClaSignedViaGitlab, gitlabAs
	case gerritAs != "":
		return models.MyClaSignedViaGerrit, gerritAs
	}
	return "", ""
}

// GetMyIdentities returns the deduplicated "<type>:<value>" identities the authenticated user
// owns - the union of their EasyCLA records and platform account, the two sources
// authorizeIdentity checks
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
	for source, byID := range platform.ids {
		for id := range byID {
			add(source+"-id", id)
		}
	}

	sort.Strings(identities)
	return &models.MyIdentityList{
		LfUsername:  currentUsername,
		Identities:  identities,
		ResultCount: int64(len(identities)),
	}, nil
}

// AuthorizeIdentity narrows the requested identity keys to those belonging to the authenticated
// user and reports the dropped ones - the boundary GET /my-clas enforces
func (s *service) AuthorizeIdentity(ctx context.Context, currentUsername string, admin bool, requested *Identity) (*Identity, []string, error) {
	return s.effectiveIdentity(ctx, &Caller{Username: currentUsername, Admin: admin}, requested)
}

// effectiveIdentity resolves which identity keys may be searched. An admin or trusted LFX Self
// Serve caller supplies them directly: a trusted list is Auth0-derived and not re-derivable here,
// as the historical GitHub-only signers this endpoint serves carry no lf_username on their EasyCLA
// records, so verifying against them would deny exactly the CLAs the caller may see. Anyone else
// has every key verified against their own records first.
func (s *service) effectiveIdentity(ctx context.Context, caller *Caller, requested *Identity) (*Identity, []string, error) {
	if caller == nil {
		return nil, nil, errors.New("no authenticated principal")
	}
	if caller.Admin || caller.Trusted {
		identity := *requested
		if identity.LfUsername == "" {
			identity.LfUsername = caller.Username
		}
		return &identity, []string{}, nil
	}
	if caller.Username == "" {
		return nil, nil, errors.New("no username on the authenticated principal")
	}
	return s.authorizeIdentity(ctx, caller.Username, requested)
}

func callerUsername(caller *Caller) string {
	if caller == nil {
		return ""
	}
	return caller.Username
}

type platformIdentitySet struct {
	emails    map[string]bool
	usernames map[string]map[string][]string
	ids       map[string]map[string]bool
}

// authorizeIdentity verifies each requested key against all of the authenticated user's own
// EasyCLA records and, when not covered there, the identities connected to their LF account -
// unverified keys are dropped and reported; verified usernames become their canonical spellings
// for the exact-match index lookups
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
			if len(variants) > 0 {
				return variants
			}
			return append(variants, platformIdentities().usernames[source][key]...)
		}
	}
	idAllowed := func(selfIDs map[string]bool, source string) func(int64) bool {
		return func(id int64) bool {
			key := strconv.FormatInt(id, 10)
			return selfIDs[key] || platformIdentities().ids[source][key]
		}
	}

	allowed := &Identity{LfUsername: currentUsername}
	skipped := []string{}

	if requested.LfUsername != "" && !strings.EqualFold(requested.LfUsername, currentUsername) {
		skipped = append(skipped, "lfUsername:"+requested.LfUsername)
	}
	appendAllowedStrings(requested.Emails, "email", normalizeEmail, emailAllowed, &allowed.Emails, &skipped)
	appendAllowedStrings(requested.SecondaryEmails, "secondaryEmail", normalizeEmail, emailAllowed, &allowed.SecondaryEmails, &skipped)
	appendAllowedIDs(requested.GithubIDs, "githubId", idAllowed(selfGithubIDs, identitySourceGithub), &allowed.GithubIDs, &skipped)
	appendAllowedUsernames(requested.GithubUsernames, "githubUsername", canonFor(identitySourceGithub), &allowed.GithubUsernames, &skipped)
	appendAllowedIDs(requested.GitlabIDs, "gitlabId", idAllowed(selfGitlabIDs, identitySourceGitlab), &allowed.GitlabIDs, &skipped)
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

func newPlatformIdentitySet() *platformIdentitySet {
	return &platformIdentitySet{
		emails: make(map[string]bool),
		usernames: map[string]map[string][]string{
			identitySourceGithub: {},
			identitySourceGitlab: {},
			identitySourceGerrit: {},
		},
		ids: map[string]map[string]bool{
			identitySourceGithub: {},
			identitySourceGitlab: {},
		},
	}
}

func (set *platformIdentitySet) merge(other *platformIdentitySet) {
	for email := range other.emails {
		set.emails[email] = true
	}
	for source, canon := range other.usernames {
		for _, variants := range canon {
			for _, variant := range variants {
				addCanonical(set.usernames[source], variant)
			}
		}
	}
	for source, ids := range other.ids {
		for id := range ids {
			set.ids[source][id] = true
		}
	}
}

// loadPlatformIdentities collects the emails and per-source canonical usernames connected to the
// LF account, fetching the Auth0 and user-service sources concurrently - a lookup failure yields
// an empty set, so affected keys are skipped, never allowed
func (s *service) loadPlatformIdentities(ctx context.Context, lfUsername string) *platformIdentitySet {
	auth0Set := newPlatformIdentitySet()
	userServiceSet := newPlatformIdentitySet()

	var eg errgroup.Group
	eg.Go(func() error {
		s.addAuth0Identities(ctx, lfUsername, auth0Set)
		return nil
	})
	eg.Go(func() error {
		s.addUserServiceIdentities(ctx, lfUsername, userServiceSet)
		return nil
	})
	_ = eg.Wait() // nolint:errcheck

	auth0Set.merge(userServiceSet)
	return auth0Set
}

// addUserServiceIdentities merges the emails and connected identities from the platform
// user-service into the set
func (s *service) addUserServiceIdentities(ctx context.Context, lfUsername string, set *platformIdentitySet) {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.service.addUserServiceIdentities",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"lfUsername":     lfUsername,
	}

	if s.platformUsersService == nil {
		return
	}

	platformUser, err := s.platformUsersService.GetUserByUsernameContext(ctx, lfUsername)
	if err != nil || platformUser == nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup the LF user in the platform user-service")
		return
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
		return
	}
	identityList, err := s.platformUsersService.ListUserIdentities(ctx, platformUser.ID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to list the LF user's connected identities")
		return
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
}

// addAuth0Identities merges the identities linked to the LF login's Auth0 user record into the
// set - lfxOne links new identities only in Auth0, so they may exist nowhere else yet
func (s *service) addAuth0Identities(ctx context.Context, lfUsername string, set *platformIdentitySet) {
	if s.auth0Identities == nil {
		return
	}
	identities, err := s.auth0Identities.UserIdentities(ctx, lfUsername)
	if err != nil {
		log.WithFields(logrus.Fields{
			"functionName":   "v2.my_clas.service.addAuth0Identities",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"lfUsername":     lfUsername,
		}).WithError(err).Warn("unable to list the LF user's Auth0 identities")
		return
	}
	for _, identity := range identities {
		if canon, ok := set.usernames[identity.Provider]; ok {
			addCanonical(canon, identity.Username)
		}
		if idSet, ok := set.ids[identity.Provider]; ok && identity.UserID != "" {
			idSet[identity.UserID] = true
		}
	}
}

func (s *service) resolveUsers(ctx context.Context, identity *Identity) ([]*v1Models.User, error) {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.service.resolveUsers",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	var lookups []userLookup
	byValue := func(values []string, what string, lookup func(context.Context, string) ([]*v1Models.User, error)) {
		for _, value := range values {
			lookups = append(lookups, userLookup{what: what, key: value, run: func(lookupCtx context.Context) ([]*v1Models.User, error) {
				return lookup(lookupCtx, value)
			}})
		}
	}
	byID := func(ids []int64, what string, lookup func(context.Context, int64) ([]*v1Models.User, error)) {
		for _, id := range dedupeIDs(ids) {
			lookups = append(lookups, userLookup{what: what, key: strconv.FormatInt(id, 10), run: func(lookupCtx context.Context) ([]*v1Models.User, error) {
				return lookup(lookupCtx, id)
			}})
		}
	}

	byValue(trimAll(append([]string{identity.LfUsername}, identity.GerritUsernames...)), "LF username", s.repo.GetUsersByLFUsername)
	byValue(normalizeEmails(identity.Emails), "email", s.repo.GetUsersByPrimaryEmail)
	if secondaryEmails := normalizeEmails(identity.SecondaryEmails); len(secondaryEmails) > 0 {
		lookups = append(lookups, userLookup{what: "secondary email", key: strings.Join(secondaryEmails, ","), run: func(lookupCtx context.Context) ([]*v1Models.User, error) {
			return s.repo.GetUsersBySecondaryEmails(lookupCtx, secondaryEmails)
		}})
	}
	byID(identity.GithubIDs, "GitHub ID", s.repo.GetUsersByGithubID)
	byValue(trimAll(identity.GithubUsernames), "GitHub username", s.repo.GetUsersByGithubUsername)
	byID(identity.GitlabIDs, "GitLab ID", s.repo.GetUsersByGitlabID)
	byValue(trimAll(identity.GitlabUsernames), "GitLab username", s.repo.GetUsersByGitlabUsername)

	matches := make([][]*v1Models.User, len(lookups))
	errs := make([]error, len(lookups))
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(fetchConcurrency)
	for i, lookup := range lookups {
		group.Go(func() error {
			matches[i], errs[i] = lookup.run(groupCtx)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	var userModels []*v1Models.User
	seen := make(map[string]bool)
	for i, lookup := range lookups {
		if errs[i] != nil {
			log.WithFields(f).WithError(errs[i]).Warnf("unable to lookup users by %s: %s", lookup.what, lookup.key)
			return nil, errs[i]
		}
		for _, userModel := range matches[i] {
			if userModel == nil || userModel.UserID == "" || seen[userModel.UserID] {
				continue
			}
			seen[userModel.UserID] = true
			userModels = append(userModels, userModel)
		}
	}

	return userModels, nil
}

// userLookup is one pending user-record lookup. All run concurrently and merge in declaration
// order, so the resolved set and the reported error match a serial walk.
type userLookup struct {
	run  func(context.Context) ([]*v1Models.User, error)
	what string
	key  string
}

// eclaCoverage is one ECLA's coverage outcome. covered drives valid; unevaluable means the
// approval-list check never completed, so a false covered proves nothing.
type eclaCoverage struct {
	covered     bool
	unevaluable bool
}

// sanctionState is the sanctions answer for one employer: the flag plus how it was obtained
type sanctionState struct {
	flagged bool
	check   string
	date    string
}

func (s *service) sanctionsMode() string {
	if s.sanctions == nil {
		return models.MyClaListSssModeDisabled
	}
	return s.sanctions.Mode()
}

// companySanctions screens one employer, live where possible. An unreadable employer is
// unavailable, never an absent answer.
func (s *service) companySanctions(ctx context.Context, companyModel *v1Models.Company, actor *v1Models.User) sanctionState {
	if companyModel == nil {
		return sanctionState{check: models.MyClaFlaggedCheckUnavailable}
	}
	state := sanctionState{flagged: companyModel.IsSanctioned, check: models.MyClaFlaggedCheckStored, date: companyModel.SanctionedDate}
	if s.sanctions != nil {
		state.flagged, state.check = s.sanctions.ScreenCompany(ctx, companyModel)
		s.persistLiveSanction(ctx, companyModel, &state, actor)
	}
	return state
}

// persistLiveSanction stamps sanctioned_date the first time a live screen flags an employer, so
// the reported date stops moving with every listing. A record already carrying the date is left
// alone - restamping it here would drift on each page view - and a failed write only costs this
// employer its stored date, never the listing.
func (s *service) persistLiveSanction(ctx context.Context, companyModel *v1Models.Company, state *sanctionState, actor *v1Models.User) {
	if !state.flagged || state.check != models.MyClaFlaggedCheckLive || (companyModel.IsSanctioned && companyModel.SanctionedDate != "") {
		return
	}
	f := logrus.Fields{
		"functionName":   "v2.my_clas.service.persistLiveSanction",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyModel.CompanyID,
	}
	newSanction := !companyModel.IsSanctioned
	if err := s.companyRepo.UpdateCompanySanctionStatus(ctx, companyModel.CompanyID, true, sanctionOriginSSS); err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to persist the live sanction for company %s - reporting the flag without a date", companyModel.CompanyID)
		// A retained date belongs to the previous, cleared sanction - drop it rather than
		// report it as this flag's date.
		state.date = ""
		return
	}
	log.WithFields(f).Warnf("live screen flagged company %s, persisted the sanction with origin=%s", companyModel.CompanyID, sanctionOriginSSS)
	_, state.date = utils.CurrentTime()
	if newSanction && s.eventsService != nil && actor != nil {
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:    events.CompanySanctioned,
			UserID:       actor.UserID,
			LfUsername:   actor.LfUsername,
			UserModel:    actor,
			CompanyModel: companyModel,
			EventData:    &events.CompanySanctionedEventData{},
		})
	}
}

// assignMyClaStatus sets the contributor-facing status independently of approved/valid. A
// sanctioned employer wins over everything else and carries no user action.
func assignMyClaStatus(row *models.MyCla, coverage eclaCoverage) {
	switch {
	case row.Flagged:
		row.Status = models.MyClaStatusRevoked
	case !row.Approved:
		row.Status = models.MyClaStatusInvalidated
	case row.ClaType == utils.ClaTypeICLA:
		row.Status = models.MyClaStatusValid
	case coverage.unevaluable:
		row.Status = models.MyClaStatusUnknown
		row.StatusReason = models.MyClaStatusReasonUnknown
	case coverage.covered:
		row.Status = models.MyClaStatusValid
	default:
		row.Status = models.MyClaStatusNeedsAttention
		row.StatusReason = models.MyClaStatusReasonNotOnApprovalList
	}
}

// evaluateApproval mirrors the PR gating logic (signatures EvaluateUserApproval): the user must
// match the current approval lists of the employer's approved+signed CCLA. A check that could not
// complete is unevaluable, so a false covered never means "no longer approved".
func (s *service) evaluateApproval(ctx context.Context, userModel *v1Models.User, ccla *v1Models.Signature) eclaCoverage {
	covered, githubOrgLookupFailed, err := s.signaturesService.EvaluateUserApproval(ctx, userModel, ccla)
	if err != nil {
		log.WithFields(logrus.Fields{
			"functionName":   "v2.my_clas.service.evaluateApproval",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     ccla.ProjectID,
			"companyID":      ccla.SignatureReferenceID,
			"userID":         userModel.UserID,
		}).WithError(err).Warn("unable to evaluate the approval list for the employee acknowledgement")
		return eclaCoverage{unevaluable: true}
	}
	// EvaluateUserApproval cannot evaluate GitLab group membership (it needs per-group OAuth
	// tokens); defer to signature_approved, which the invalidation flow maintains. Membership was
	// never checked, so the row stays unevaluable.
	if !covered && len(ccla.GitlabOrgApprovalList) > 0 {
		return eclaCoverage{covered: true, unevaluable: true}
	}
	return eclaCoverage{covered: covered, unevaluable: githubOrgLookupFailed}
}

func (s *service) claGroupName(ctx context.Context, claGroupID string) (string, error) {
	if claGroupID == "" {
		return "", nil
	}
	name, err := s.projectsClaGroupsRepo.GetCLAGroupNameByID(ctx, claGroupID)
	if err != nil {
		if !errors.Is(err, projects_cla_groups.ErrCLAGroupDoesNotExist) {
			return "", err
		}
		return "", nil
	}
	return name, nil
}

// projectInfo resolves the Salesforce project display name, logo, and ids of a CLA Group. The
// name comes from the projects-cla-groups mapping, the logo only from the project-service,
// fetched by project SFID (a foundation-level CLA Group resolves to its foundation). A lookup
// miss degrades to an empty logo rather than failing the listing. The ids are the ones the
// Corporate Console path needs; they are omitted when the mapping is unresolved.
func (s *service) projectInfo(ctx context.Context, claGroupID string) (projectInfo, error) {
	if claGroupID == "" {
		return projectInfo{}, nil
	}

	mappings, err := s.projectsClaGroupsRepo.GetProjectsIdsForClaGroup(ctx, claGroupID)
	if err != nil {
		return projectInfo{}, err
	}

	var info projectInfo
	var lookupSFID string
	// A foundation-level CLA Group is marked by a mapping with ProjectSFID == FoundationSFID (the
	// projects_cla_groups convention used by SignedAtFoundation), not by the mapping count, and
	// resolves to its foundation; a single project-level mapping resolves to that project. Several
	// project-level mappings with no foundation marker stay unresolved (empty name/logo/ids, so the
	// consumer falls back to claGroupName) rather than picking an arbitrary one.
	switch fm := foundationMapping(mappings); {
	case fm != nil:
		lookupSFID = fm.FoundationSFID
		info.foundationSFID = fm.FoundationSFID
		info.name = fm.FoundationName
	case len(mappings) == 1:
		lookupSFID = mappings[0].ProjectSFID
		info.projectSFID = mappings[0].ProjectSFID
		info.foundationSFID = mappings[0].FoundationSFID
		info.name = mappings[0].ProjectName
	}

	if lookupSFID != "" && s.projectService != nil {
		f := logrus.Fields{
			"functionName":   "v2.my_clas.service.projectInfo",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     claGroupID,
			"lookupSFID":     lookupSFID,
		}
		project, projectErr := s.projectService.GetProject(lookupSFID)
		if projectErr != nil {
			log.WithFields(f).WithError(projectErr).Warn("unable to load the project details for the CLA group - leaving the logo empty")
		} else if project != nil {
			if project.Name != "" {
				info.name = project.Name
			}
			info.logo = project.ProjectLogo
		}
	}

	return info, nil
}

// foundationMapping returns the mapping marking a foundation-level CLA Group (ProjectSFID ==
// FoundationSFID, the SignedAtFoundation convention), or nil when it is not foundation-level.
func foundationMapping(mappings []*projects_cla_groups.ProjectClaGroup) *projects_cla_groups.ProjectClaGroup {
	for _, m := range mappings {
		if m.FoundationSFID != "" && m.FoundationSFID == m.ProjectSFID {
			return m
		}
	}
	return nil
}

func (s *service) company(ctx context.Context, companyID string) (*v1Models.Company, error) {
	companyModel, err := s.companyRepo.GetCompany(ctx, companyID)
	if err != nil {
		var companyNotFound *utils.CompanyNotFound
		if !errors.As(err, &companyNotFound) {
			return nil, err
		}
		return nil, nil
	}
	return companyModel, nil
}

// signedOn mirrors the v1 signatures converter: prefer signed_on, fall back to date_created,
// then date_modified
func signedOn(sig *signatures.ItemSignature) string {
	value := sig.SignedOn
	if value == "" {
		value = sig.DateCreated
	}
	if value == "" {
		value = sig.DateModified
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
