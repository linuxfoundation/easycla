// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package self_serve_sign

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	goapierrors "github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	githubsdk "github.com/google/go-github/v37/github"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/my_clas"
	"github.com/stretchr/testify/assert"
)

type fakeMyClas struct {
	allowed *my_clas.Identity
	skipped []string
	err     error
}

func (f *fakeMyClas) AuthorizeIdentity(_ context.Context, currentUsername string, _ bool, _ *my_clas.Identity) (*my_clas.Identity, []string, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	allowed := *f.allowed
	if allowed.LfUsername == "" {
		allowed.LfUsername = currentUsername
	}
	return &allowed, append([]string{}, f.skipped...), nil
}

type fakeUsers struct {
	byGithubID       map[string]*v1Models.User
	byGithubUsername map[string]*v1Models.User
	byGitlabID       map[int]*v1Models.User
	byGitlabUsername map[string]*v1Models.User
	byLFUsername     map[string]*v1Models.User
	byEmail          map[string]*v1Models.User
	created          *v1Models.User
	updates          map[string]interface{}
	createErr        error
	notFound         func() error
	lookupErr        error
}

var errNotFound = errors.New("not found")

func (f *fakeUsers) GetUserByGitHubID(gitHubID string) (*v1Models.User, error) {
	return lookupIn(f, f.byGithubID, gitHubID)
}
func (f *fakeUsers) GetUserByGitHubUsername(gitHubUsername string) (*v1Models.User, error) {
	return lookupIn(f, f.byGithubUsername, gitHubUsername)
}
func (f *fakeUsers) GetUserByGitlabID(gitLabID int) (*v1Models.User, error) {
	return lookupIn(f, f.byGitlabID, gitLabID)
}
func (f *fakeUsers) GetUserByGitLabUsername(gitLabUsername string) (*v1Models.User, error) {
	return lookupIn(f, f.byGitlabUsername, gitLabUsername)
}
func (f *fakeUsers) GetUserByLFUserName(lfUserName string) (*v1Models.User, error) {
	return lookupIn(f, f.byLFUsername, lfUserName)
}
func (f *fakeUsers) GetUserByEmail(userEmail string) (*v1Models.User, error) {
	return lookupIn(f, f.byEmail, userEmail)
}

func (f *fakeUsers) CreateUser(userModel *v1Models.User, _ *user.CLAUser) (*v1Models.User, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	created := *userModel
	created.UserID = "created-user-id"
	f.created = &created
	return &created, nil
}

func (f *fakeUsers) UpdateUser(userID string, updates map[string]interface{}) (*v1Models.User, error) {
	f.updates = updates
	return &v1Models.User{UserID: userID}, nil
}

func lookupIn[K comparable](f *fakeUsers, values map[K]*v1Models.User, key K) (*v1Models.User, error) {
	if f.lookupErr != nil {
		return nil, f.lookupErr
	}
	if found, ok := values[key]; ok {
		return found, nil
	}
	if f.notFound != nil {
		return nil, f.notFound()
	}
	return nil, goapierrors.NotFound("user not found")
}

type fakeCLAGroups struct {
	claGroup *v1Models.ClaGroup
	err      error
}

func (f *fakeCLAGroups) GetCLAGroupByID(_ context.Context, _ string) (*v1Models.ClaGroup, error) {
	return f.claGroup, f.err
}

type fakeProjectsCLAGroups struct {
	projects []*projects_cla_groups.ProjectClaGroup
	err      error
}

func (f *fakeProjectsCLAGroups) GetProjectsIdsForClaGroup(_ context.Context, _ string) ([]*projects_cla_groups.ProjectClaGroup, error) {
	return f.projects, f.err
}

type fakeStore struct {
	key   string
	value string
	err   error
}

func (f *fakeStore) SetActiveSignatureMetaData(_ context.Context, key string, _ int64, value string) error {
	f.key, f.value = key, value
	return f.err
}

const testCLAGroupID = "aa47b3e1-6f9c-4b6a-9f16-0f9d6a2e1c11"

func newTestService(myClas MyClasService, users UsersService, claGroups CLAGroupService, store StoreRepository) *service {
	return &service{
		myClasService:         myClas,
		usersService:          users,
		claGroupService:       claGroups,
		projectsClaGroupsRepo: &fakeProjectsCLAGroups{projects: []*projects_cla_groups.ProjectClaGroup{{ProjectSFID: "a09P000000DsCE6IAN"}}},
		storeRepo:             store,
		contributorConsoleURL: "contributor.dev.lfx.linuxfoundation.org",
		githubUserDetails: func(username string) (*githubsdk.User, error) {
			if username != "octocat" {
				return nil, errNotFound
			}
			id := int64(26589865)
			return &githubsdk.User{ID: &id}, nil
		},
	}
}

func enabledCLAGroup() *fakeCLAGroups {
	return &fakeCLAGroups{claGroup: &v1Models.ClaGroup{
		ProjectID:          testCLAGroupID,
		ProjectName:        "Test CLA Group",
		FoundationSFID:     "a09P000000DsCE5IAN",
		ProjectICLAEnabled: true,
		ProjectCCLAEnabled: true,
	}}
}

func stringRef(value string) *string { return &value }

func uriRef(value string) *strfmt.URI {
	uri := strfmt.URI(value)
	return &uri
}

const testReturnURL = "https://openprofile.dev/my-clas"

func TestPrepareSignResolvesExistingUserByGithubID(t *testing.T) {
	existing := &v1Models.User{UserID: "existing-user-id", LfUsername: "lgryglicki", GithubID: "26589865", GithubUsername: "octocat"}
	users := &fakeUsers{byGithubID: map[string]*v1Models.User{"26589865": existing}}
	store := &fakeStore{}
	svc := newTestService(
		&fakeMyClas{allowed: &my_clas.Identity{GithubIDs: []int64{26589865}, GithubUsernames: []string{"octocat"}}},
		users, enabledCLAGroup(), store)

	result, err := svc.PrepareSign(context.Background(), "lgryglicki", "l@example.org", false, &models.PrepareSignInput{
		ClaGroupID:     stringRef(testCLAGroupID),
		ReturnURL:      uriRef(testReturnURL),
		GithubID:       26589865,
		GithubUsername: "octocat",
	})

	assert.NoError(t, err)
	assert.Equal(t, "existing-user-id", result.UserID)
	assert.False(t, result.UserCreated)
	assert.Nil(t, users.created)
	assert.Equal(t, "a09P000000DsCE6IAN", result.ProjectSfid)
	assert.Contains(t, result.Identity, "github-id:26589865")
	assert.Equal(t, "https://contributor.dev.lfx.linuxfoundation.org/#/cla/project/"+testCLAGroupID+"/user/existing-user-id?redirect=https%3A%2F%2Fopenprofile.dev%2Fmy-clas", result.SignURL)

	assert.Equal(t, "active_signature:existing-user-id", store.key)
	var metadata map[string]interface{}
	assert.NoError(t, json.Unmarshal([]byte(store.value), &metadata))
	assert.Equal(t, "self-serve", metadata["source"])
	assert.Equal(t, testCLAGroupID, metadata["project_id"])
	assert.Equal(t, "https://openprofile.dev/my-clas", metadata["return_url"])
}

func TestPrepareSignCreatesUserForFirstTimeSigner(t *testing.T) {
	users := &fakeUsers{}
	svc := newTestService(
		&fakeMyClas{allowed: &my_clas.Identity{GithubUsernames: []string{"octocat"}}, skipped: []string{"githubId:26589865"}},
		users, enabledCLAGroup(), &fakeStore{})

	result, err := svc.PrepareSign(context.Background(), "lgryglicki", "l@example.org", false, &models.PrepareSignInput{
		ClaGroupID:     stringRef(testCLAGroupID),
		ReturnURL:      uriRef(testReturnURL),
		GithubID:       26589865,
		GithubUsername: "octocat",
	})

	assert.NoError(t, err)
	assert.True(t, result.UserCreated)
	assert.Equal(t, "created-user-id", result.UserID)
	assert.Equal(t, "26589865", users.created.GithubID)
	assert.Equal(t, "octocat", users.created.GithubUsername)
	assert.Equal(t, "lgryglicki", users.created.LfUsername)
	assert.Equal(t, "l@example.org", string(users.created.LfEmail))
	assert.Empty(t, result.SkippedIdentities)
	assert.True(t, strings.HasSuffix(result.SignURL, "?redirect=https%3A%2F%2Fopenprofile.dev%2Fmy-clas"))
}

func TestPrepareSignKeepsUnmatchedGithubIDSkipped(t *testing.T) {
	users := &fakeUsers{}
	svc := newTestService(
		&fakeMyClas{allowed: &my_clas.Identity{GithubUsernames: []string{"octocat"}}, skipped: []string{"githubId:999"}},
		users, enabledCLAGroup(), &fakeStore{})

	result, err := svc.PrepareSign(context.Background(), "lgryglicki", "", false, &models.PrepareSignInput{
		ClaGroupID:     stringRef(testCLAGroupID),
		ReturnURL:      uriRef(testReturnURL),
		GithubID:       999,
		GithubUsername: "octocat",
	})

	assert.NoError(t, err)
	assert.Equal(t, []string{"githubId:999"}, result.SkippedIdentities)
	assert.Empty(t, users.created.GithubID)
}

func TestPrepareSignIgnoresARecordBoundToAnotherGithubID(t *testing.T) {
	recycled := &v1Models.User{UserID: "previous-owner-id", GithubUsername: "octocat", GithubID: "999"}
	users := &fakeUsers{byGithubUsername: map[string]*v1Models.User{"octocat": recycled}}
	svc := newTestService(
		&fakeMyClas{allowed: &my_clas.Identity{GithubUsernames: []string{"octocat"}}},
		users, enabledCLAGroup(), &fakeStore{})

	result, err := svc.PrepareSign(context.Background(), "lgryglicki", "", false, &models.PrepareSignInput{
		ClaGroupID:     stringRef(testCLAGroupID),
		ReturnURL:      uriRef(testReturnURL),
		GithubID:       26589865,
		GithubUsername: "octocat",
	})

	assert.NoError(t, err)
	assert.True(t, result.UserCreated)
	assert.Equal(t, "created-user-id", result.UserID)
	assert.Equal(t, "26589865", users.created.GithubID)
}

func TestPrepareSignRejectsUnverifiedIdentity(t *testing.T) {
	svc := newTestService(
		&fakeMyClas{allowed: &my_clas.Identity{}, skipped: []string{"githubId:26589865", "githubUsername:octocat"}},
		&fakeUsers{}, enabledCLAGroup(), &fakeStore{})

	_, err := svc.PrepareSign(context.Background(), "lgryglicki", "", false, &models.PrepareSignInput{
		ClaGroupID:     stringRef(testCLAGroupID),
		ReturnURL:      uriRef(testReturnURL),
		GithubID:       26589865,
		GithubUsername: "octocat",
	})

	assert.ErrorIs(t, err, ErrIdentityNotVerified)
}

func TestPrepareSignRejectsAnotherLFUsername(t *testing.T) {
	svc := newTestService(
		&fakeMyClas{allowed: &my_clas.Identity{LfUsername: "lgryglicki"}, skipped: []string{"lfUsername:someone-else"}},
		&fakeUsers{}, enabledCLAGroup(), &fakeStore{})

	_, err := svc.PrepareSign(context.Background(), "lgryglicki", "", false, &models.PrepareSignInput{
		ClaGroupID: stringRef(testCLAGroupID),
		ReturnURL:  uriRef(testReturnURL),
		LfUsername: "someone-else",
	})

	assert.ErrorIs(t, err, ErrIdentityNotVerified)
}

func TestPrepareSignDefaultsToTheAuthenticatedLFUsername(t *testing.T) {
	existing := &v1Models.User{UserID: "existing-user-id", LfUsername: "lgryglicki"}
	svc := newTestService(
		&fakeMyClas{allowed: &my_clas.Identity{}},
		&fakeUsers{byLFUsername: map[string]*v1Models.User{"lgryglicki": existing}}, enabledCLAGroup(), &fakeStore{})

	result, err := svc.PrepareSign(context.Background(), "lgryglicki", "", false, &models.PrepareSignInput{
		ClaGroupID: stringRef(testCLAGroupID),
		ReturnURL:  uriRef(testReturnURL),
	})

	assert.NoError(t, err)
	assert.Equal(t, "existing-user-id", result.UserID)
	assert.Contains(t, result.Identity, "lf-username:lgryglicki")
}

func TestPrepareSignEnrichesOnlyMissingIdentityFields(t *testing.T) {
	existing := &v1Models.User{UserID: "existing-user-id", LfUsername: "lgryglicki", GithubID: "111"}
	users := &fakeUsers{byGithubID: map[string]*v1Models.User{"26589865": existing}}
	svc := newTestService(
		&fakeMyClas{allowed: &my_clas.Identity{GithubIDs: []int64{26589865}, GithubUsernames: []string{"octocat"}}},
		users, enabledCLAGroup(), &fakeStore{})

	_, err := svc.PrepareSign(context.Background(), "lgryglicki", "", false, &models.PrepareSignInput{
		ClaGroupID:     stringRef(testCLAGroupID),
		ReturnURL:      uriRef(testReturnURL),
		GithubID:       26589865,
		GithubUsername: "octocat",
	})

	assert.NoError(t, err)
	assert.Equal(t, "octocat", users.updates["user_github_username"])
	_, updatedGithubID := users.updates["user_github_id"]
	assert.False(t, updatedGithubID)
}

func TestPrepareSignUnknownCLAGroup(t *testing.T) {
	svc := newTestService(&fakeMyClas{allowed: &my_clas.Identity{}}, &fakeUsers{},
		&fakeCLAGroups{err: errNotFound}, &fakeStore{})

	_, err := svc.PrepareSign(context.Background(), "lgryglicki", "", false, &models.PrepareSignInput{
		ClaGroupID: stringRef(testCLAGroupID),
		ReturnURL:  uriRef(testReturnURL),
	})

	assert.ErrorIs(t, err, ErrCLAGroupNotFound)
}

func TestPrepareSignSigningNotEnabled(t *testing.T) {
	svc := newTestService(&fakeMyClas{allowed: &my_clas.Identity{}}, &fakeUsers{},
		&fakeCLAGroups{claGroup: &v1Models.ClaGroup{ProjectID: testCLAGroupID}}, &fakeStore{})

	_, err := svc.PrepareSign(context.Background(), "lgryglicki", "", false, &models.PrepareSignInput{
		ClaGroupID: stringRef(testCLAGroupID),
		ReturnURL:  uriRef(testReturnURL),
	})

	assert.ErrorIs(t, err, ErrSigningNotEnabled)
}

func TestPrepareSignRequiresAnIdentityForAnAdminWithoutAPrincipal(t *testing.T) {
	svc := newTestService(&fakeMyClas{allowed: &my_clas.Identity{}}, &fakeUsers{}, enabledCLAGroup(), &fakeStore{})

	_, err := svc.PrepareSign(context.Background(), "", "", true, &models.PrepareSignInput{
		ClaGroupID: stringRef(testCLAGroupID),
		ReturnURL:  uriRef(testReturnURL),
	})

	assert.ErrorIs(t, err, ErrIdentityRequired)
}

func TestPrepareSignStoresProviderIDsAsNumbers(t *testing.T) {
	existing := &v1Models.User{UserID: "existing-user-id", LfUsername: "lgryglicki", GithubUsername: "octocat"}
	users := &fakeUsers{byGithubUsername: map[string]*v1Models.User{"octocat": existing}}
	svc := newTestService(
		&fakeMyClas{allowed: &my_clas.Identity{GithubIDs: []int64{26589865}, GithubUsernames: []string{"octocat"}, GitlabIDs: []int64{77}}},
		users, enabledCLAGroup(), &fakeStore{})

	_, err := svc.PrepareSign(context.Background(), "lgryglicki", "", false, &models.PrepareSignInput{
		ClaGroupID:     stringRef(testCLAGroupID),
		ReturnURL:      uriRef(testReturnURL),
		GithubID:       26589865,
		GithubUsername: "octocat",
	})

	assert.NoError(t, err)
	// the user table and its GSIs key both provider IDs as DynamoDB numbers
	assert.Equal(t, int64(26589865), users.updates["user_github_id"])
	assert.Equal(t, int64(77), users.updates["user_gitlab_id"])
}

func TestPrepareSignRecordsTheVerifiedIdentityACL(t *testing.T) {
	sessionACL := func(t *testing.T, allowed *my_clas.Identity, input *models.PrepareSignInput) string {
		t.Helper()
		store := &fakeStore{}
		svc := newTestService(&fakeMyClas{allowed: allowed}, &fakeUsers{}, enabledCLAGroup(), store)
		_, err := svc.PrepareSign(context.Background(), "lgryglicki", "", false, input)
		assert.NoError(t, err)
		var metadata map[string]interface{}
		assert.NoError(t, json.Unmarshal([]byte(store.value), &metadata))
		acl, ok := metadata["acl"].(string)
		assert.True(t, ok)
		return acl
	}

	assert.Equal(t, "github:26589865", sessionACL(t,
		&my_clas.Identity{GithubIDs: []int64{26589865}, GithubUsernames: []string{"octocat"}},
		&models.PrepareSignInput{ClaGroupID: stringRef(testCLAGroupID), ReturnURL: uriRef(testReturnURL), GithubID: 26589865, GithubUsername: "octocat"}))

	assert.Equal(t, "gitlab:77", sessionACL(t,
		&my_clas.Identity{GitlabIDs: []int64{77}, GitlabUsernames: []string{"lgryglicki"}},
		&models.PrepareSignInput{ClaGroupID: stringRef(testCLAGroupID), ReturnURL: uriRef(testReturnURL), GitlabID: 77, GitlabUsername: "lgryglicki"}))

	assert.Equal(t, "lgryglicki", sessionACL(t,
		&my_clas.Identity{LfUsername: "lgryglicki"},
		&models.PrepareSignInput{ClaGroupID: stringRef(testCLAGroupID), ReturnURL: uriRef(testReturnURL), LfUsername: "lgryglicki"}))
}

func TestPrepareSignDoesNotBindTheAdminIdentityToAnotherContributor(t *testing.T) {
	admin := &v1Models.User{UserID: "admin-user-id", LfUsername: "lfadmin", LfEmail: "admin@example.org"}
	users := &fakeUsers{byLFUsername: map[string]*v1Models.User{"lfadmin": admin}}
	svc := newTestService(
		&fakeMyClas{allowed: &my_clas.Identity{GithubIDs: []int64{26589865}, GithubUsernames: []string{"octocat"}}},
		users, enabledCLAGroup(), &fakeStore{})

	result, err := svc.PrepareSign(context.Background(), "lfadmin", "admin@example.org", true, &models.PrepareSignInput{
		ClaGroupID:     stringRef(testCLAGroupID),
		ReturnURL:      uriRef(testReturnURL),
		GithubID:       26589865,
		GithubUsername: "octocat",
	})

	assert.NoError(t, err)
	assert.Equal(t, "created-user-id", result.UserID)
	assert.True(t, result.UserCreated)
	assert.Nil(t, users.updates)
	assert.NotNil(t, users.created)
	assert.Equal(t, "", users.created.LfUsername)
	assert.Equal(t, "", string(users.created.LfEmail))
	assert.Empty(t, users.created.Emails)
	assert.Equal(t, "octocat", users.created.Username)
}

func TestPrepareSignLetsAnAdminPrepareForThemselves(t *testing.T) {
	admin := &v1Models.User{UserID: "admin-user-id", LfUsername: "lfadmin"}
	users := &fakeUsers{byLFUsername: map[string]*v1Models.User{"lfadmin": admin}}
	svc := newTestService(&fakeMyClas{allowed: &my_clas.Identity{LfUsername: "lfadmin"}}, users, enabledCLAGroup(), &fakeStore{})

	result, err := svc.PrepareSign(context.Background(), "lfadmin", "admin@example.org", true, &models.PrepareSignInput{
		ClaGroupID: stringRef(testCLAGroupID),
		ReturnURL:  uriRef(testReturnURL),
		LfUsername: "lfadmin",
	})

	assert.NoError(t, err)
	assert.Equal(t, "admin-user-id", result.UserID)
	assert.Nil(t, users.created)
}

func TestPrepareSignRejectsANonHTTPSReturnURL(t *testing.T) {
	for _, returnURL := range []string{"http://openprofile.dev/my-clas", "javascript:alert(1)", "/my-clas", "https://"} {
		svc := newTestService(&fakeMyClas{allowed: &my_clas.Identity{LfUsername: "lgryglicki"}}, &fakeUsers{}, enabledCLAGroup(), &fakeStore{})

		_, err := svc.PrepareSign(context.Background(), "lgryglicki", "", false, &models.PrepareSignInput{
			ClaGroupID: stringRef(testCLAGroupID),
			ReturnURL:  uriRef(returnURL),
		})

		assert.ErrorIs(t, err, ErrReturnURLNotSupported, returnURL)
	}
}

func TestPrepareSignDoesNotCreateAUserWhenTheLookupFails(t *testing.T) {
	users := &fakeUsers{lookupErr: errors.New("dynamodb is unavailable")}
	svc := newTestService(
		&fakeMyClas{allowed: &my_clas.Identity{GithubUsernames: []string{"octocat"}}},
		users, enabledCLAGroup(), &fakeStore{})

	_, err := svc.PrepareSign(context.Background(), "lgryglicki", "l@example.org", false, &models.PrepareSignInput{
		ClaGroupID:     stringRef(testCLAGroupID),
		ReturnURL:      uriRef(testReturnURL),
		GithubUsername: "octocat",
	})

	assert.Error(t, err)
	assert.Nil(t, users.created)
}

func TestPrepareSignTreatsEveryNotFoundShapeAsAMiss(t *testing.T) {
	shapes := map[string]func() error{
		"go-openapi not found": func() error { return goapierrors.NotFound("user not found") },
		"utils.UserNotFound":   func() error { return &utils.UserNotFound{Message: "user not found"} },
		"nil user":             func() error { return nil },
	}

	for name, shape := range shapes {
		t.Run(name, func(t *testing.T) {
			users := &fakeUsers{notFound: shape, byEmail: map[string]*v1Models.User{"l@example.org": {UserID: "existing-user-id"}}}
			svc := newTestService(
				&fakeMyClas{allowed: &my_clas.Identity{GithubUsernames: []string{"octocat"}, Emails: []string{"l@example.org"}}},
				users, enabledCLAGroup(), &fakeStore{})

			result, err := svc.PrepareSign(context.Background(), "lgryglicki", "l@example.org", false, &models.PrepareSignInput{
				ClaGroupID:     stringRef(testCLAGroupID),
				ReturnURL:      uriRef(testReturnURL),
				GithubUsername: "octocat",
				Email:          "l@example.org",
			})

			assert.NoError(t, err)
			assert.False(t, result.UserCreated)
			assert.Equal(t, "existing-user-id", result.UserID)
			assert.Nil(t, users.created)
		})
	}
}
