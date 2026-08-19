// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package self_serve_sign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/strfmt"
	githubsdk "github.com/google/go-github/v37/github"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/my_clas"
	"github.com/sirupsen/logrus"
)

// ErrIdentityRequired is returned when the request carries no identity to sign under
var ErrIdentityRequired = errors.New("no identity provided - provide at least one of lfUsername, email, githubId, githubUsername, gitlabId, gitlabUsername, gerritUsername")

// ErrIdentityNotVerified is returned when the provided identity could not be verified as belonging to the authenticated user
var ErrIdentityNotVerified = errors.New("the provided identity does not belong to the authenticated user")

// ErrCLAGroupNotFound is returned when the CLA Group does not exist
var ErrCLAGroupNotFound = errors.New("cla group not found")

// ErrSigningNotEnabled is returned when the CLA Group offers neither an ICLA nor a CCLA
var ErrSigningNotEnabled = errors.New("the cla group has neither an individual nor a corporate CLA enabled")

const activeSignatureTTLDays = 1

// MyClasService is the subset of the My CLAs service used to verify identity ownership
type MyClasService interface {
	AuthorizeIdentity(ctx context.Context, currentUsername string, admin bool, requested *my_clas.Identity) (*my_clas.Identity, []string, error)
}

// UsersService is the subset of the users service used to resolve, enrich and create EasyCLA user records
type UsersService interface {
	GetUserByLFUserName(lfUserName string) (*v1Models.User, error)
	GetUserByEmail(userEmail string) (*v1Models.User, error)
	GetUserByGitHubID(gitHubID string) (*v1Models.User, error)
	GetUserByGitHubUsername(gitHubUsername string) (*v1Models.User, error)
	GetUserByGitlabID(gitLabID int) (*v1Models.User, error)
	GetUserByGitLabUsername(gitLabUsername string) (*v1Models.User, error)
	CreateUser(userModel *v1Models.User, claUser *user.CLAUser) (*v1Models.User, error)
	UpdateUser(userID string, updates map[string]interface{}) (*v1Models.User, error)
}

// CLAGroupService is the subset of the CLA Group service used to resolve the selected CLA Group
type CLAGroupService interface {
	GetCLAGroupByID(ctx context.Context, claGroupID string) (*v1Models.ClaGroup, error)
}

// ProjectsCLAGroupsRepository is the subset of the projects-cla-groups repository used to resolve the Salesforce IDs of a CLA Group
type ProjectsCLAGroupsRepository interface {
	GetProjectsIdsForClaGroup(ctx context.Context, claGroupID string) ([]*projects_cla_groups.ProjectClaGroup, error)
}

// StoreRepository is the subset of the store repository used to record the active signing session
type StoreRepository interface {
	SetActiveSignatureMetaData(ctx context.Context, key string, expire int64, value string) error
}

// Service interface defines the Self Serve signing service methods
type Service interface {
	PrepareSign(ctx context.Context, currentUsername, currentEmail string, admin bool, input *models.PrepareSignInput) (*models.PrepareSign, error)
}

type service struct {
	myClasService         MyClasService
	usersService          UsersService
	claGroupService       CLAGroupService
	projectsClaGroupsRepo ProjectsCLAGroupsRepository
	storeRepo             StoreRepository
	contributorConsoleURL string
	githubUserDetails     func(username string) (*githubsdk.User, error)
}

// NewService creates a new instance of the Self Serve signing service
func NewService(myClasService MyClasService, usersService UsersService, claGroupService CLAGroupService, projectsClaGroupsRepo ProjectsCLAGroupsRepository, storeRepo StoreRepository, contributorConsoleURL string) Service {
	return &service{
		myClasService:         myClasService,
		usersService:          usersService,
		claGroupService:       claGroupService,
		projectsClaGroupsRepo: projectsClaGroupsRepo,
		storeRepo:             storeRepo,
		contributorConsoleURL: contributorConsoleURL,
		githubUserDetails:     github.GetUserDetails,
	}
}

// PrepareSign verifies the requested identity belongs to the authenticated user, resolves or
// creates the EasyCLA user record for it, records the signing session and returns the
// Contributor Console hand-off URL
func (s *service) PrepareSign(ctx context.Context, currentUsername, currentEmail string, admin bool, input *models.PrepareSignInput) (*models.PrepareSign, error) {
	claGroupID := strings.TrimSpace(utils.StringValue(input.ClaGroupID))
	f := logrus.Fields{
		"functionName":    "v2.self_serve_sign.service.PrepareSign",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"currentUsername": currentUsername,
		"claGroupID":      claGroupID,
	}

	claGroup, err := s.claGroupService.GetCLAGroupByID(ctx, claGroupID)
	if err != nil || claGroup == nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup the cla group")
		return nil, ErrCLAGroupNotFound
	}
	if !claGroup.ProjectICLAEnabled && !claGroup.ProjectCCLAEnabled {
		log.WithFields(f).Warn(ErrSigningNotEnabled.Error())
		return nil, ErrSigningNotEnabled
	}

	requested := identityFromInput(input)
	if requested.IsEmpty() {
		if currentUsername == "" {
			return nil, ErrIdentityRequired
		}
		requested.LfUsername = currentUsername
	}

	allowed, skipped, err := s.myClasService.AuthorizeIdentity(ctx, currentUsername, admin, requested)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to verify the provided identity")
		return nil, err
	}
	skipped = s.acceptVerifiedGithubID(ctx, input, allowed, skipped)
	if !identityAccepted(requested, allowed) {
		log.WithFields(f).WithField("skippedIdentities", skipped).Warn(ErrIdentityNotVerified.Error())
		return nil, ErrIdentityNotVerified
	}

	userModel, created, err := s.resolveOrCreateUser(ctx, allowed, currentUsername, currentEmail)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to resolve or create the EasyCLA user record")
		return nil, err
	}

	returnURL := ""
	if input.ReturnURL != nil {
		returnURL = strings.TrimSpace(input.ReturnURL.String())
	}
	if err := s.recordSigningSession(ctx, userModel.UserID, claGroupID, returnURL); err != nil {
		log.WithFields(f).WithError(err).Warn("unable to record the active signing session")
		return nil, err
	}

	result := &models.PrepareSign{
		UserID:            userModel.UserID,
		UserCreated:       created,
		LfUsername:        userModel.LfUsername,
		UserName:          userModel.Username,
		UserEmail:         string(userModel.LfEmail),
		Identity:          identityKeys(allowed),
		SkippedIdentities: skipped,
		ClaGroupID:        claGroupID,
		ClaGroupName:      claGroup.ProjectName,
		FoundationSfid:    claGroup.FoundationSFID,
		IclaEnabled:       claGroup.ProjectICLAEnabled,
		CclaEnabled:       claGroup.ProjectCCLAEnabled,
		CclaRequiresIcla:  claGroup.ProjectCCLARequiresICLA,
		ReturnURL:         returnURL,
		SignURL:           s.consoleSignURL(claGroupID, userModel.UserID, returnURL),
	}
	if result.UserEmail == "" && len(userModel.Emails) > 0 {
		result.UserEmail = userModel.Emails[0]
	}
	result.ProjectSfid = s.projectSFID(ctx, claGroupID, claGroup.ProjectExternalID)

	return result, nil
}

// acceptVerifiedGithubID admits the requested GitHub numeric ID when it resolves to the GitHub
// account named by an already verified GitHub username - the platform user-service exposes
// usernames only, so a first-time signer's numeric ID cannot be verified any other way
func (s *service) acceptVerifiedGithubID(ctx context.Context, input *models.PrepareSignInput, allowed *my_clas.Identity, skipped []string) []string {
	if input.GithubID <= 0 || containsID(allowed.GithubIDs, input.GithubID) {
		return skipped
	}
	f := logrus.Fields{
		"functionName":   "v2.self_serve_sign.service.acceptVerifiedGithubID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"githubID":       input.GithubID,
		"githubUsername": input.GithubUsername,
	}

	username := strings.TrimSpace(input.GithubUsername)
	if username == "" || !containsFold(allowed.GithubUsernames, username) {
		return skipped
	}
	githubUser, err := s.githubUserDetails(username)
	if err != nil || githubUser == nil || githubUser.GetID() != input.GithubID {
		log.WithFields(f).WithError(err).Warn("the provided GitHub ID does not match the verified GitHub username")
		return skipped
	}

	allowed.GithubIDs = append(allowed.GithubIDs, input.GithubID)
	return removeValue(skipped, "githubId:"+strconv.FormatInt(input.GithubID, 10))
}

func (s *service) resolveOrCreateUser(ctx context.Context, allowed *my_clas.Identity, currentUsername, currentEmail string) (*v1Models.User, bool, error) {
	userModel := s.resolveUser(ctx, allowed)
	if userModel != nil {
		return s.enrichUser(ctx, userModel, allowed), false, nil
	}

	newUser := &v1Models.User{
		LfUsername: allowed.LfUsername,
		Username:   allowed.LfUsername,
	}
	if len(allowed.GithubIDs) > 0 {
		newUser.GithubID = strconv.FormatInt(allowed.GithubIDs[0], 10)
	}
	if len(allowed.GithubUsernames) > 0 {
		newUser.GithubUsername = allowed.GithubUsernames[0]
		newUser.Username = allowed.GithubUsernames[0]
	}
	if len(allowed.GitlabIDs) > 0 {
		newUser.GitlabID = strconv.FormatInt(allowed.GitlabIDs[0], 10)
	}
	if len(allowed.GitlabUsernames) > 0 {
		newUser.GitlabUsername = allowed.GitlabUsernames[0]
	}
	if email := firstValue(append(allowed.Emails, currentEmail)); email != "" {
		newUser.LfEmail = strfmt.Email(email)
		newUser.Emails = []string{email}
	}
	if newUser.Username == "" {
		newUser.Username = string(newUser.LfEmail)
	}

	created, err := s.usersService.CreateUser(newUser, &user.CLAUser{LFUsername: currentUsername})
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

func (s *service) resolveUser(ctx context.Context, allowed *my_clas.Identity) *v1Models.User {
	f := logrus.Fields{
		"functionName":   "v2.self_serve_sign.service.resolveUser",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	for _, githubID := range allowed.GithubIDs {
		if found, err := s.usersService.GetUserByGitHubID(strconv.FormatInt(githubID, 10)); err == nil && found != nil {
			return found
		}
	}
	for _, githubUsername := range allowed.GithubUsernames {
		found, err := s.usersService.GetUserByGitHubUsername(githubUsername)
		if err != nil || found == nil {
			continue
		}
		if !idBelongs(found.GithubID, allowed.GithubIDs) {
			log.WithFields(f).Warnf("skipping user record %s matched on github username %s - stored github id %s is not one of the verified ids %v", found.UserID, githubUsername, found.GithubID, allowed.GithubIDs)
			continue
		}
		return found
	}
	for _, gitlabID := range allowed.GitlabIDs {
		if found, err := s.usersService.GetUserByGitlabID(int(gitlabID)); err == nil && found != nil {
			return found
		}
	}
	for _, gitlabUsername := range allowed.GitlabUsernames {
		found, err := s.usersService.GetUserByGitLabUsername(gitlabUsername)
		if err != nil || found == nil {
			continue
		}
		if !idBelongs(found.GitlabID, allowed.GitlabIDs) {
			log.WithFields(f).Warnf("skipping user record %s matched on gitlab username %s - stored gitlab id %s is not one of the verified ids %v", found.UserID, gitlabUsername, found.GitlabID, allowed.GitlabIDs)
			continue
		}
		return found
	}
	for _, lfUsername := range append([]string{allowed.LfUsername}, allowed.GerritUsernames...) {
		if strings.TrimSpace(lfUsername) == "" {
			continue
		}
		if found, err := s.usersService.GetUserByLFUserName(lfUsername); err == nil && found != nil {
			return found
		}
	}
	for _, email := range allowed.Emails {
		if found, err := s.usersService.GetUserByEmail(email); err == nil && found != nil {
			return found
		}
	}

	log.WithFields(f).Debug("no EasyCLA user record matched the verified identity")
	return nil
}

// enrichUser fills in the verified identity fields the matched record is missing - an existing
// value is never replaced, so a record already bound to another linked identity is left alone
func (s *service) enrichUser(ctx context.Context, userModel *v1Models.User, allowed *my_clas.Identity) *v1Models.User {
	updates := make(map[string]interface{})
	if userModel.LfUsername == "" && allowed.LfUsername != "" {
		updates["lf_username"] = allowed.LfUsername
	}
	if userModel.GithubID == "" && len(allowed.GithubIDs) > 0 {
		updates["user_github_id"] = strconv.FormatInt(allowed.GithubIDs[0], 10)
	}
	if userModel.GithubUsername == "" && len(allowed.GithubUsernames) > 0 {
		updates["user_github_username"] = allowed.GithubUsernames[0]
	}
	if userModel.GitlabID == "" && len(allowed.GitlabIDs) > 0 {
		updates["user_gitlab_id"] = strconv.FormatInt(allowed.GitlabIDs[0], 10)
	}
	if userModel.GitlabUsername == "" && len(allowed.GitlabUsernames) > 0 {
		updates["user_gitlab_username"] = allowed.GitlabUsernames[0]
	}
	if len(updates) == 0 {
		return userModel
	}

	updates["date_modified"] = time.Now().UTC().Format(time.RFC3339)
	updated, err := s.usersService.UpdateUser(userModel.UserID, updates)
	if err != nil || updated == nil {
		log.WithFields(logrus.Fields{
			"functionName":   "v2.self_serve_sign.service.enrichUser",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"userID":         userModel.UserID,
		}).WithError(err).Warn("unable to store the verified identity on the EasyCLA user record")
		return userModel
	}
	return updated
}

type activeSignatureMetadata struct {
	UserID    string `json:"user_id"`
	ProjectID string `json:"project_id"`
	ReturnURL string `json:"return_url,omitempty"`
	Source    string `json:"source"`
}

func (s *service) recordSigningSession(ctx context.Context, userID, claGroupID, returnURL string) error {
	value, err := json.Marshal(&activeSignatureMetadata{
		UserID:    userID,
		ProjectID: claGroupID,
		ReturnURL: returnURL,
		Source:    utils.SelfServeSignatureSource,
	})
	if err != nil {
		return err
	}
	expire := time.Now().AddDate(0, 0, activeSignatureTTLDays).Unix()
	return s.storeRepo.SetActiveSignatureMetaData(ctx, fmt.Sprintf("active_signature:%s", userID), expire, string(value))
}

func (s *service) consoleSignURL(claGroupID, userID, returnURL string) string {
	signURL := fmt.Sprintf("https://%s/#/cla/project/%s/user/%s", strings.TrimSuffix(s.contributorConsoleURL, "/"), claGroupID, userID)
	if returnURL != "" {
		signURL += "?redirect=" + url.QueryEscape(returnURL)
	}
	return signURL
}

func (s *service) projectSFID(ctx context.Context, claGroupID, fallback string) string {
	projects, err := s.projectsClaGroupsRepo.GetProjectsIdsForClaGroup(ctx, claGroupID)
	if err != nil {
		log.WithFields(logrus.Fields{
			"functionName":   "v2.self_serve_sign.service.projectSFID",
			utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
			"claGroupID":     claGroupID,
		}).WithError(err).Warn("unable to resolve the Salesforce projects of the cla group")
		return fallback
	}
	if len(projects) == 1 {
		return projects[0].ProjectSFID
	}
	return ""
}

func identityFromInput(input *models.PrepareSignInput) *my_clas.Identity {
	identity := &my_clas.Identity{LfUsername: strings.TrimSpace(input.LfUsername)}
	appendValue(&identity.Emails, input.Email)
	appendValue(&identity.GithubUsernames, input.GithubUsername)
	appendValue(&identity.GitlabUsernames, input.GitlabUsername)
	appendValue(&identity.GerritUsernames, input.GerritUsername)
	if input.GithubID > 0 {
		identity.GithubIDs = []int64{input.GithubID}
	}
	if input.GitlabID > 0 {
		identity.GitlabIDs = []int64{input.GitlabID}
	}
	return identity
}

// identityAccepted requires the identity actually asked for to have survived verification - a
// requested key that was dropped must not silently fall back to signing as the LF username
func identityAccepted(requested, allowed *my_clas.Identity) bool {
	if allowed == nil {
		return false
	}
	if requested.LfUsername != "" && !strings.EqualFold(requested.LfUsername, allowed.LfUsername) {
		return false
	}
	if onlyLfUsername(requested) {
		return strings.TrimSpace(allowed.LfUsername) != ""
	}
	return len(allowed.Emails) > 0 || len(allowed.GithubIDs) > 0 || len(allowed.GithubUsernames) > 0 ||
		len(allowed.GitlabIDs) > 0 || len(allowed.GitlabUsernames) > 0 || len(allowed.GerritUsernames) > 0
}

func onlyLfUsername(identity *my_clas.Identity) bool {
	lfUsername := identity.LfUsername
	identity.LfUsername = ""
	empty := identity.IsEmpty()
	identity.LfUsername = lfUsername
	return empty
}

func identityKeys(allowed *my_clas.Identity) []string {
	keys := []string{}
	if allowed.LfUsername != "" {
		keys = append(keys, "lf-username:"+allowed.LfUsername)
	}
	for _, email := range allowed.Emails {
		keys = append(keys, "email:"+email)
	}
	for _, githubID := range allowed.GithubIDs {
		keys = append(keys, "github-id:"+strconv.FormatInt(githubID, 10))
	}
	for _, githubUsername := range allowed.GithubUsernames {
		keys = append(keys, "github-username:"+githubUsername)
	}
	for _, gitlabID := range allowed.GitlabIDs {
		keys = append(keys, "gitlab-id:"+strconv.FormatInt(gitlabID, 10))
	}
	for _, gitlabUsername := range allowed.GitlabUsernames {
		keys = append(keys, "gitlab-username:"+gitlabUsername)
	}
	for _, gerritUsername := range allowed.GerritUsernames {
		keys = append(keys, "gerrit-username:"+gerritUsername)
	}
	return keys
}

func appendValue(values *[]string, value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		*values = append(*values, trimmed)
	}
}

func firstValue(values []string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// idBelongs guards the username lookups - provider usernames are recyclable, so a record whose
// stored numeric ID is not one of the verified ones belongs to a previous owner of that username
func idBelongs(storedID string, verifiedIDs []int64) bool {
	storedID = strings.TrimSpace(storedID)
	if storedID == "" {
		return true
	}
	parsed, err := strconv.ParseInt(storedID, 10, 64)
	if err != nil {
		return false
	}
	return containsID(verifiedIDs, parsed)
}

func containsID(ids []int64, id int64) bool {
	for _, value := range ids {
		if value == id {
			return true
		}
	}
	return false
}

func containsFold(values []string, value string) bool {
	for _, item := range values {
		if strings.EqualFold(item, value) {
			return true
		}
	}
	return false
}

func removeValue(values []string, value string) []string {
	result := []string{}
	for _, item := range values {
		if item != value {
			result = append(result, item)
		}
	}
	return result
}
