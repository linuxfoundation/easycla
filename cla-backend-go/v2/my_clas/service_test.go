// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	v2ProjectServiceModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service/models"
	platformModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	byUserID         map[string][]*signatures.ItemSignature
	byLFUsername     map[string][]*v1Models.User
	byPrimaryEmail   map[string][]*v1Models.User
	byGithubID       map[int64][]*v1Models.User
	byGithubUsername map[string][]*v1Models.User
	byGitlabID       map[int64][]*v1Models.User
	byGitlabUsername map[string][]*v1Models.User
	bySecondaryEmail map[string][]*v1Models.User
	secondaryScans   int
}

func (f *fakeRepo) GetUserCLASignatures(_ context.Context, userID string) ([]*signatures.ItemSignature, error) {
	return f.byUserID[userID], nil
}

func (f *fakeRepo) GetUsersByLFUsername(_ context.Context, lfUsername string) ([]*v1Models.User, error) {
	return f.byLFUsername[lfUsername], nil
}

func (f *fakeRepo) GetUsersByPrimaryEmail(_ context.Context, email string) ([]*v1Models.User, error) {
	return f.byPrimaryEmail[email], nil
}

func (f *fakeRepo) GetUsersByGithubID(_ context.Context, githubID int64) ([]*v1Models.User, error) {
	return f.byGithubID[githubID], nil
}

func (f *fakeRepo) GetUsersByGithubUsername(_ context.Context, githubUsername string) ([]*v1Models.User, error) {
	return f.byGithubUsername[githubUsername], nil
}

func (f *fakeRepo) GetUsersByGitlabID(_ context.Context, gitlabID int64) ([]*v1Models.User, error) {
	return f.byGitlabID[gitlabID], nil
}

func (f *fakeRepo) GetUsersByGitlabUsername(_ context.Context, gitlabUsername string) ([]*v1Models.User, error) {
	return f.byGitlabUsername[gitlabUsername], nil
}

func (f *fakeRepo) GetUsersBySecondaryEmails(_ context.Context, emails []string) ([]*v1Models.User, error) {
	f.secondaryScans++
	var matches []*v1Models.User
	for _, email := range emails {
		matches = append(matches, f.bySecondaryEmail[email]...)
	}
	return matches, nil
}

type fakePlatform struct {
	user       *platformModels.User
	identities []*platformModels.UserIdentity
	lookups    int
}

func (f *fakePlatform) GetUserByUsernameContext(_ context.Context, _ string) (*platformModels.User, error) {
	f.lookups++
	if f.user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return f.user, nil
}

func (f *fakePlatform) ListUserIdentities(_ context.Context, _ string) ([]*platformModels.UserIdentity, error) {
	return f.identities, nil
}

type fakeSignatures struct {
	cclas             map[string]*v1Models.Signature
	approvedUserIDs   map[string]bool
	userIsApprovedErr error
	cclaErr           error
	mu                sync.Mutex
	orgLookupFailed   bool
	cclaCalls         int
}

func (f *fakeSignatures) GetCorporateSignature(_ context.Context, claGroupID, companyID string, _, _ *bool) (*v1Models.Signature, error) {
	f.mu.Lock()
	f.cclaCalls++
	f.mu.Unlock()
	if f.cclaErr != nil {
		return nil, f.cclaErr
	}
	return f.cclas[claGroupID+"|"+companyID], nil
}

func (f *fakeSignatures) EvaluateUserApproval(_ context.Context, user *v1Models.User, _ *v1Models.Signature) (bool, bool, error) {
	if f.userIsApprovedErr != nil {
		return false, false, f.userIsApprovedErr
	}
	return f.approvedUserIDs[user.UserID], f.orgLookupFailed, nil
}

type sanctionWrite struct {
	companyID  string
	sanctioned bool
	origin     string
}

type fakeCompanies struct {
	byID     map[string]*v1Models.Company
	failIDs  map[string]bool
	mu       sync.Mutex
	calls    int
	writes   []sanctionWrite
	writeErr error
}

func (f *fakeCompanies) GetCompany(_ context.Context, companyID string) (*v1Models.Company, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()
	if f.failIDs[companyID] {
		return nil, fmt.Errorf("dynamodb unavailable for company %s", companyID)
	}
	if companyModel, ok := f.byID[companyID]; ok {
		return companyModel, nil
	}
	return nil, &utils.CompanyNotFound{CompanyID: companyID}
}

func (f *fakeCompanies) UpdateCompanySanctionStatus(_ context.Context, companyID string, sanctioned bool, origin string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writeErr != nil {
		return f.writeErr
	}
	f.writes = append(f.writes, sanctionWrite{companyID: companyID, sanctioned: sanctioned, origin: origin})
	return nil
}

// fakeScreener stands in for the live SSS screen and records how often each employer was screened
type fakeScreener struct {
	mode    string
	flagged map[string]bool
	checks  map[string]string
	calls   map[string]int
	mu      sync.Mutex
}

func (f *fakeScreener) Mode() string {
	return f.mode
}

func (f *fakeScreener) ScreenCompany(_ context.Context, company *v1Models.Company) (bool, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[company.CompanyID]++
	check, ok := f.checks[company.CompanyID]
	if !ok {
		check = models.MyClaFlaggedCheckLive
	}
	return f.flagged[company.CompanyID], check
}

type fakeClaGroups struct {
	names    map[string]string
	mappings map[string][]*projects_cla_groups.ProjectClaGroup
}

func (f *fakeClaGroups) GetCLAGroupNameByID(_ context.Context, claGroupID string) (string, error) {
	if name, ok := f.names[claGroupID]; ok {
		return name, nil
	}
	return "", projects_cla_groups.ErrCLAGroupDoesNotExist
}

func (f *fakeClaGroups) GetProjectsIdsForClaGroup(_ context.Context, claGroupID string) ([]*projects_cla_groups.ProjectClaGroup, error) {
	return f.mappings[claGroupID], nil
}

type fakeProjectService struct {
	byID  map[string]*v2ProjectServiceModels.ProjectOutputDetailed
	err   error
	calls map[string]int
	mu    sync.Mutex
}

func (f *fakeProjectService) GetProject(projectSFID string) (*v2ProjectServiceModels.ProjectOutputDetailed, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls != nil {
		f.calls[projectSFID]++
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.byID[projectSFID], nil
}

func icla(signatureID, userID, claGroupID, signedOn string, approved bool) *signatures.ItemSignature {
	return &signatures.ItemSignature{
		SignatureID:                   signatureID,
		SignatureProjectID:            claGroupID,
		SignatureReferenceID:          userID,
		SignatureReferenceType:        utils.SignatureReferenceTypeUser,
		SignatureType:                 utils.SignatureTypeCLA,
		SignatureSigned:               true,
		SignatureApproved:             approved,
		SignedOn:                      signedOn,
		SignatureDocumentMajorVersion: 2,
	}
}

// ECLAs auto-created from approval-list changes carry signature_type=ecla while
// DocuSign-era ECLAs carry signature_type=cla - classification must not depend on it
func ecla(signatureID, companyID, signedOn string, approved bool) *signatures.ItemSignature {
	sig := icla(signatureID, "user-a", "cla-group-1", signedOn, approved)
	sig.SignatureUserCompanyID = companyID
	sig.SignatureType = utils.ClaTypeECLA
	return sig
}

func newTestService(repo Repository, platform PlatformUsersService, signaturesService SignaturesService, companyRepo CompanyRepository, claGroups ProjectsCLAGroupsRepository) *service {
	return &service{
		repo:                  repo,
		platformUsersService:  platform,
		signaturesService:     signaturesService,
		companyRepo:           companyRepo,
		projectsClaGroupsRepo: claGroups,
		presign: func(filename string) (string, error) {
			return "https://s3.example.org/" + filename, nil
		},
		documentExists: func(_ string) (bool, error) {
			return true, nil
		},
	}
}

func TestGetMyClasUnionAndDedupe(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone", LfEmail: "someone@example.org", GithubID: "12345"}
	userB := &v1Models.User{UserID: "user-b", GithubID: "12345"}

	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {icla("sig-1", "user-a", "cla-group-1", "2024-01-02T00:00:00Z", true)},
			"user-b": {icla("sig-2", "user-b", "cla-group-1", "2025-01-02T00:00:00Z", true)},
		},
		byLFUsername:   map[string][]*v1Models.User{"someone": {userA}},
		byPrimaryEmail: map[string][]*v1Models.User{"someone@example.org": {userA, userB}},
		byGithubID:     map[int64][]*v1Models.User{12345: {userB}},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{names: map[string]string{"cla-group-1": "My CLA Group"}})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{
		Emails:    []string{"Someone@Example.org ", "someone@example.org"},
		GithubIDs: []int64{12345, 12345},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"user-a", "user-b"}, result.UserIds)
	assert.Empty(t, result.SkippedIdentities)
	assert.Equal(t, int64(2), result.ResultCount)
	require.Len(t, result.Clas, 2)
	assert.Equal(t, "sig-2", result.Clas[0].SignatureID)
	assert.Equal(t, "sig-1", result.Clas[1].SignatureID)
	assert.Equal(t, "My CLA Group", result.Clas[0].ClaGroupName)
}

func TestGetMyClasProjectNameAndLogo(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {
				icla("sig-single", "user-a", "cla-group-1", "2024-02-01T00:00:00Z", true),
				icla("sig-foundation", "user-a", "cla-group-2", "2024-01-01T00:00:00Z", true),
			},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	claGroups := &fakeClaGroups{
		names: map[string]string{"cla-group-1": "Kubernetes CLA Group", "cla-group-2": "CNCF CLA Group"},
		mappings: map[string][]*projects_cla_groups.ProjectClaGroup{
			// single-project CLA Group -> resolves to the project SFID
			"cla-group-1": {{ClaGroupID: "cla-group-1", ProjectSFID: "proj-sfid-1", ProjectName: "Kubernetes"}},
			// foundation-level CLA Group -> identified by the marker row (ProjectSFID == FoundationSFID)
			// and resolves to the foundation SFID
			"cla-group-2": {
				{ClaGroupID: "cla-group-2", ProjectSFID: "found-sfid", FoundationSFID: "found-sfid", FoundationName: "CNCF", ProjectName: "CNCF"},
				{ClaGroupID: "cla-group-2", ProjectSFID: "proj-sfid-2b", FoundationSFID: "found-sfid", FoundationName: "CNCF"},
			},
		},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, claGroups)
	svc.projectService = &fakeProjectService{byID: map[string]*v2ProjectServiceModels.ProjectOutputDetailed{
		"proj-sfid-1": {ProjectOutput: v2ProjectServiceModels.ProjectOutput{ProjectCommon: v2ProjectServiceModels.ProjectCommon{Name: "Kubernetes", ProjectLogo: "https://logos.example.org/k8s.png"}}},
		"found-sfid":  {ProjectOutput: v2ProjectServiceModels.ProjectOutput{ProjectCommon: v2ProjectServiceModels.ProjectCommon{Name: "Cloud Native Computing Foundation", ProjectLogo: "https://logos.example.org/cncf.png"}}},
	}}

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err)
	require.Len(t, result.Clas, 2)

	byID := map[string]models.MyCla{}
	for _, row := range result.Clas {
		byID[row.SignatureID] = row
	}

	single := byID["sig-single"]
	assert.Equal(t, "Kubernetes CLA Group", single.ClaGroupName, "the CLA group name is the subtext")
	assert.Equal(t, "Kubernetes", single.ProjectName, "the project-service name wins over the mapping-table name")
	assert.Equal(t, "https://logos.example.org/k8s.png", single.ProjectLogo)

	foundation := byID["sig-foundation"]
	assert.Equal(t, "CNCF CLA Group", foundation.ClaGroupName)
	assert.Equal(t, "Cloud Native Computing Foundation", foundation.ProjectName, "a foundation-level CLA group resolves to its foundation")
	assert.Equal(t, "https://logos.example.org/cncf.png", foundation.ProjectLogo)
}

func TestGetMyClasProjectLookupDegradesGracefully(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {icla("sig-1", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", true)},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	// mapping table carries the project name but the project-service has no logo for the SFID
	claGroups := &fakeClaGroups{
		names: map[string]string{"cla-group-1": "Kubernetes CLA Group"},
		mappings: map[string][]*projects_cla_groups.ProjectClaGroup{
			"cla-group-1": {{ClaGroupID: "cla-group-1", ProjectSFID: "proj-sfid-1", ProjectName: "Kubernetes"}},
		},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, claGroups)
	svc.projectService = &fakeProjectService{byID: map[string]*v2ProjectServiceModels.ProjectOutputDetailed{}}

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err, "a project-service miss must not fail the listing")
	require.Len(t, result.Clas, 1)
	assert.Equal(t, "Kubernetes", result.Clas[0].ProjectName, "the mapping-table name is kept when the project-service has no record")
	assert.Empty(t, result.Clas[0].ProjectLogo, "a missing project logo degrades to empty")
}

// A CLA Group mapped to several projects with no foundation-marker row (no mapping where
// ProjectSFID == FoundationSFID) is NOT foundation-level and has no single project the
// signature represents, so the project fields are left empty (the consumer falls back to
// claGroupName) rather than being branded with an arbitrary one of the mapped projects.
func TestGetMyClasMultiProjectNonFoundation(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {icla("sig-1", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", true)},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	claGroups := &fakeClaGroups{
		names: map[string]string{"cla-group-1": "Multi CLA Group"},
		mappings: map[string][]*projects_cla_groups.ProjectClaGroup{
			"cla-group-1": {
				{ClaGroupID: "cla-group-1", ProjectSFID: "proj-zeta", FoundationSFID: "found-x", FoundationName: "Umbrella", ProjectName: "Zeta"},
				{ClaGroupID: "cla-group-1", ProjectSFID: "proj-alpha", FoundationSFID: "found-x", FoundationName: "Umbrella", ProjectName: "Alpha"},
			},
		},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, claGroups)
	svc.projectService = &fakeProjectService{byID: map[string]*v2ProjectServiceModels.ProjectOutputDetailed{
		"proj-alpha": {ProjectOutput: v2ProjectServiceModels.ProjectOutput{ProjectCommon: v2ProjectServiceModels.ProjectCommon{Name: "Alpha", ProjectLogo: "https://logos.example.org/alpha.png"}}},
	}}

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err)
	require.Len(t, result.Clas, 1)
	assert.Empty(t, result.Clas[0].ProjectName, "an ambiguous multi-project non-foundation group invents no project name")
	assert.Empty(t, result.Clas[0].ProjectLogo, "an ambiguous multi-project non-foundation group invents no project logo")
	assert.Equal(t, "Multi CLA Group", result.Clas[0].ClaGroupName, "the consumer falls back to claGroupName")
}

// The resolved project metadata is cached per request: several signatures for the same CLA
// Group trigger only one project-service lookup.
func TestGetMyClasProjectCacheHitPerRequest(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {
				icla("sig-1", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", true),
				icla("sig-2", "user-a", "cla-group-1", "2024-02-01T00:00:00Z", true),
			},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	claGroups := &fakeClaGroups{
		names: map[string]string{"cla-group-1": "Kubernetes CLA Group"},
		mappings: map[string][]*projects_cla_groups.ProjectClaGroup{
			"cla-group-1": {{ClaGroupID: "cla-group-1", ProjectSFID: "proj-sfid-1", ProjectName: "Kubernetes"}},
		},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, claGroups)
	projectSvc := &fakeProjectService{
		byID:  map[string]*v2ProjectServiceModels.ProjectOutputDetailed{"proj-sfid-1": {ProjectOutput: v2ProjectServiceModels.ProjectOutput{ProjectCommon: v2ProjectServiceModels.ProjectCommon{Name: "Kubernetes", ProjectLogo: "https://logos.example.org/k8s.png"}}}},
		calls: map[string]int{},
	}
	svc.projectService = projectSvc

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err)
	require.Len(t, result.Clas, 2)
	assert.Equal(t, 1, projectSvc.calls["proj-sfid-1"], "the project-service is queried once per distinct CLA group within a request")
}

// Both an actual project-service error and a nil project-service client are non-fatal: the
// listing succeeds, the mapping-table name is kept, and the logo degrades to empty.
func TestGetMyClasProjectServiceErrorAndNilClient(t *testing.T) {
	newRepo := func() *fakeRepo {
		userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
		return &fakeRepo{
			byUserID: map[string][]*signatures.ItemSignature{
				"user-a": {icla("sig-1", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", true)},
			},
			byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
		}
	}
	newClaGroups := func() *fakeClaGroups {
		return &fakeClaGroups{
			names: map[string]string{"cla-group-1": "Kubernetes CLA Group"},
			mappings: map[string][]*projects_cla_groups.ProjectClaGroup{
				"cla-group-1": {{ClaGroupID: "cla-group-1", ProjectSFID: "proj-sfid-1", ProjectName: "Kubernetes"}},
			},
		}
	}

	t.Run("project-service returns an error", func(t *testing.T) {
		svc := newTestService(newRepo(), &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, newClaGroups())
		svc.projectService = &fakeProjectService{err: errors.New("project-service unavailable")}

		result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
		require.NoError(t, err, "a project-service error must not fail the listing")
		require.Len(t, result.Clas, 1)
		assert.Equal(t, "Kubernetes", result.Clas[0].ProjectName, "the mapping-table name is kept on a project-service error")
		assert.Empty(t, result.Clas[0].ProjectLogo, "the logo degrades to empty on a project-service error")
	})

	t.Run("nil project-service client", func(t *testing.T) {
		svc := newTestService(newRepo(), &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, newClaGroups())
		svc.projectService = nil

		result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
		require.NoError(t, err, "a nil project-service client must not fail the listing")
		require.Len(t, result.Clas, 1)
		assert.Equal(t, "Kubernetes", result.Clas[0].ProjectName, "the mapping-table name is kept with no project-service client")
		assert.Empty(t, result.Clas[0].ProjectLogo, "the logo degrades to empty with no project-service client")
	})
}

func TestGetMyClasMultipleRecordsSameLFID(t *testing.T) {
	userA1 := &v1Models.User{UserID: "user-a1", LfUsername: "alice"}
	userA2 := &v1Models.User{UserID: "user-a2", LfUsername: "alice", GithubID: "12345"}
	userA3 := &v1Models.User{UserID: "user-a3", GithubID: "12345"}

	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a1": {icla("sig-a1", "user-a1", "cla-group-1", "2024-01-01T00:00:00Z", true)},
			"user-a2": {icla("sig-a2", "user-a2", "cla-group-1", "2024-02-01T00:00:00Z", true)},
			"user-a3": {icla("sig-a3", "user-a3", "cla-group-1", "2024-03-01T00:00:00Z", true)},
		},
		byLFUsername: map[string][]*v1Models.User{"alice": {userA1, userA2}},
		byGithubID:   map[int64][]*v1Models.User{12345: {userA2, userA3}},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "alice"}, &Identity{GithubIDs: []int64{12345}})
	require.NoError(t, err)
	assert.Empty(t, result.SkippedIdentities, "a numeric ID stored on any of the caller's LFID records is authorized")
	assert.ElementsMatch(t, []string{"user-a1", "user-a2", "user-a3"}, result.UserIds, "all records per key are unioned")
	assert.Equal(t, int64(3), result.ResultCount)

	pdf, err := svc.GetMyClaPdfURL(context.Background(), &Caller{Username: "alice"}, &Identity{}, "sig-a2")
	require.NoError(t, err)
	require.NotNil(t, pdf, "a PDF owned by the second LFID record is downloadable")
}

func TestGetMyClasNoMatches(t *testing.T) {
	svc := newTestService(&fakeRepo{}, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "missing"}, &Identity{})
	require.NoError(t, err)
	assert.Empty(t, result.UserIds)
	assert.Empty(t, result.Clas)
	assert.Equal(t, int64(0), result.ResultCount)
}

func TestGetMyClasOwnershipRejectsForeignIdentities(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone", LfEmail: "someone@example.org", GithubID: "12345", GithubUsername: "someone-gh"}
	victim := &v1Models.User{UserID: "user-v", LfUsername: "victim"}

	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {icla("sig-own", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", true)},
			"user-v": {icla("sig-victim", "user-v", "cla-group-1", "2024-02-01T00:00:00Z", true)},
		},
		byLFUsername:     map[string][]*v1Models.User{"someone": {userA}, "victim": {victim}},
		byPrimaryEmail:   map[string][]*v1Models.User{"victim@example.org": {victim}},
		byGithubID:       map[int64][]*v1Models.User{999: {victim}},
		byGithubUsername: map[string][]*v1Models.User{"victim-gh": {victim}},
		byGitlabID:       map[int64][]*v1Models.User{7: {victim}},
		byGitlabUsername: map[string][]*v1Models.User{"victim-gl": {victim}},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{
		LfUsername:      "victim",
		Emails:          []string{"victim@example.org"},
		SecondaryEmails: []string{"victim-alt@example.org"},
		GithubIDs:       []int64{999},
		GithubUsernames: []string{"victim-gh"},
		GitlabIDs:       []int64{7},
		GitlabUsernames: []string{"victim-gl"},
		GerritUsernames: []string{"victim"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"user-a"}, result.UserIds, "only the caller's own record is searched")
	require.Len(t, result.Clas, 1)
	assert.Equal(t, "sig-own", result.Clas[0].SignatureID)
	assert.ElementsMatch(t, []string{
		"lfUsername:victim",
		"email:victim@example.org",
		"secondaryEmail:victim-alt@example.org",
		"githubId:999",
		"githubUsername:victim-gh",
		"gitlabId:7",
		"gitlabUsername:victim-gl",
		"gerritUsername:victim",
	}, result.SkippedIdentities)
	assert.Equal(t, 0, repo.secondaryScans)
}

func TestGetMyClasOwnershipViaEasyCLARecord(t *testing.T) {
	userA := &v1Models.User{
		UserID: "user-a", LfUsername: "someone", LfEmail: "someone@example.org",
		Emails:   []string{"alt@example.org", "alt2@example.org"},
		GitlabID: "777",
	}
	userB := &v1Models.User{UserID: "user-b"}
	userC := &v1Models.User{UserID: "user-c"}

	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-b": {icla("sig-b", "user-b", "cla-group-1", "2024-01-01T00:00:00Z", true)},
			"user-c": {icla("sig-c", "user-c", "cla-group-1", "2024-02-01T00:00:00Z", true)},
		},
		byLFUsername:     map[string][]*v1Models.User{"someone": {userA}},
		byGitlabID:       map[int64][]*v1Models.User{777: {userC}},
		bySecondaryEmail: map[string][]*v1Models.User{"alt@example.org": {userB}},
	}
	platform := &fakePlatform{}
	svc := newTestService(repo, platform, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{
		SecondaryEmails: []string{"Alt@Example.org", "alt2@example.org", "alt@example.org"},
		GitlabIDs:       []int64{777},
	})
	require.NoError(t, err)
	assert.Empty(t, result.SkippedIdentities)
	assert.ElementsMatch(t, []string{"user-a", "user-b", "user-c"}, result.UserIds)
	assert.Equal(t, 1, repo.secondaryScans, "one scan for all secondary emails")
	assert.Equal(t, 0, platform.lookups, "user-service is not consulted when the EasyCLA records cover the identities")
}

func TestGetMyClasOwnershipViaPlatformIdentities(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	userGh := &v1Models.User{UserID: "user-gh"}
	userGerrit := &v1Models.User{UserID: "user-gerrit"}

	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-gh":     {icla("sig-gh", "user-gh", "cla-group-1", "2024-01-01T00:00:00Z", true)},
			"user-gerrit": {icla("sig-gerrit", "user-gerrit", "cla-group-1", "2024-02-01T00:00:00Z", true)},
		},
		byLFUsername:     map[string][]*v1Models.User{"someone": {userA}, "old-ldap-id": {userGerrit}},
		byGithubUsername: map[string][]*v1Models.User{"Octocat": {userGh}},
	}
	platform := &fakePlatform{
		user: &platformModels.User{ID: "sfid-1", Username: "someone"},
		identities: []*platformModels.UserIdentity{
			{Source: "github", Username: "Octocat", Email: "someone-gh@example.org"},
			{Source: "gerrit", Username: "old-ldap-id"},
			{Source: "slack", Username: "not-a-code-identity"},
			{Source: "github", Username: "lakecat", DataSource: "datalake"},
		},
	}
	svc := newTestService(repo, platform, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{
		GithubUsernames: []string{"octocat"},
		GerritUsernames: []string{"old-ldap-id"},
	})
	require.NoError(t, err)
	assert.Empty(t, result.SkippedIdentities)
	assert.ElementsMatch(t, []string{"user-a", "user-gh", "user-gerrit"}, result.UserIds,
		"the canonical spelling from user-service finds records stored with exact-match keys")
	assert.Equal(t, 1, platform.lookups, "platform identities are loaded once")

	result, err = svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{
		GithubUsernames: []string{"not-a-code-identity"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"githubUsername:not-a-code-identity"}, result.SkippedIdentities,
		"a slack username must not authorize a github search")

	result, err = svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{
		GithubUsernames: []string{"lakecat"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"githubUsername:lakecat"}, result.SkippedIdentities,
		"a non-platform (datalake) identity must not authorize a search")
}

func TestGetMyClasAdminBypass(t *testing.T) {
	victim := &v1Models.User{UserID: "user-v", LfUsername: "victim"}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-v": {icla("sig-victim", "user-v", "cla-group-1", "2024-02-01T00:00:00Z", true)},
		},
		byLFUsername: map[string][]*v1Models.User{"victim": {victim}},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "staff-admin", Admin: true}, &Identity{LfUsername: "victim"})
	require.NoError(t, err)
	assert.Empty(t, result.SkippedIdentities)
	assert.Equal(t, "victim", result.LfUsername)
	assert.Equal(t, []string{"user-v"}, result.UserIds)
}

// A trusted Self Serve caller's identity list is taken as authorized - the records it names
// typically carry no LF username at all (historical GitHub-only signers), which is exactly the
// case the per-identity verification cannot authorize
func TestGetMyClasTrustedCallerBypass(t *testing.T) {
	githubOnly := &v1Models.User{UserID: "user-gh", GithubID: "999", GithubUsername: "octocat"}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-gh": {icla("sig-gh", "user-gh", "cla-group-1", "2024-02-01T00:00:00Z", true)},
		},
		byGithubID:       map[int64][]*v1Models.User{999: {githubOnly}},
		byGithubUsername: map[string][]*v1Models.User{"octocat": {githubOnly}},
	}
	platform := &fakePlatform{}
	svc := newTestService(repo, platform, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Trusted: true}, &Identity{
		GithubIDs:       []int64{999},
		GithubUsernames: []string{"octocat"},
	})
	require.NoError(t, err)
	assert.Empty(t, result.SkippedIdentities)
	assert.Equal(t, []string{"user-gh"}, result.UserIds)
	require.Len(t, result.Clas, 1)
	assert.Equal(t, "sig-gh", result.Clas[0].SignatureID)
	assert.Zero(t, platform.lookups, "a trusted caller's identity list is not verified against the platform user-service")

	pdf, err := svc.GetMyClaPdfURL(context.Background(), &Caller{Trusted: true}, &Identity{GithubIDs: []int64{999}}, "sig-gh")
	require.NoError(t, err)
	require.NotNil(t, pdf)
	assert.Equal(t, "sig-gh", pdf.SignatureID)
}

func TestEffectiveIdentityRequiresACaller(t *testing.T) {
	svc := newTestService(&fakeRepo{}, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	_, err := svc.GetMyClas(context.Background(), nil, &Identity{GithubIDs: []int64{999}})
	assert.Error(t, err, "a nil caller must never be treated as authorized")

	_, err = svc.GetMyClas(context.Background(), &Caller{}, &Identity{GithubIDs: []int64{999}})
	assert.Error(t, err, "an untrusted caller without a username must never be treated as authorized")

	_, err = svc.GetMyClaPdfURL(context.Background(), &Caller{}, &Identity{GithubIDs: []int64{999}}, "sig-1")
	assert.Error(t, err)
}

func TestEffectiveIdentityForPrivilegedCallers(t *testing.T) {
	platform := &fakePlatform{}
	svc := newTestService(&fakeRepo{}, platform, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	tests := []struct {
		name       string
		caller     *Caller
		requested  *Identity
		lfUsername string
	}{
		{"trusted caller keeps the requested lfUsername", &Caller{Username: "ss-service", Trusted: true}, &Identity{LfUsername: "someone", GithubIDs: []int64{999}}, "someone"},
		{"trusted caller without one falls back to its own username", &Caller{Username: "someone", Trusted: true}, &Identity{GithubIDs: []int64{999}}, "someone"},
		{"trusted caller with neither stays empty", &Caller{Trusted: true}, &Identity{GithubIDs: []int64{999}}, ""},
		{"admin keeps the requested lfUsername", &Caller{Username: "staff-admin", Admin: true}, &Identity{LfUsername: "victim", GithubIDs: []int64{999}}, "victim"},
		{"admin and trusted at once", &Caller{Username: "staff-admin", Admin: true, Trusted: true}, &Identity{GithubIDs: []int64{999}}, "staff-admin"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			identity, skipped, err := svc.effectiveIdentity(context.Background(), test.caller, test.requested)
			require.NoError(t, err)
			assert.Equal(t, test.lfUsername, identity.LfUsername)
			assert.Empty(t, skipped)
			assert.Equal(t, test.requested.GithubIDs, identity.GithubIDs, "the requested keys must pass through untouched")
		})
	}

	requested := &Identity{GithubIDs: []int64{999}}
	_, _, err := svc.effectiveIdentity(context.Background(), &Caller{Username: "someone", Trusted: true}, requested)
	require.NoError(t, err)
	assert.Empty(t, requested.LfUsername, "the caller's identity list must not be mutated in place")
	assert.Zero(t, platform.lookups, "a privileged caller's identity list is never verified against the platform user-service")
}

func TestIdentitySummary(t *testing.T) {
	identity := &Identity{
		LfUsername:      "someone",
		Emails:          []string{"someone@example.org", " ", "someone@example.org"},
		GithubIDs:       []int64{999, 999},
		GithubUsernames: []string{"octocat"},
	}
	assert.Equal(t, "lfUsername:someone email:someone@example.org githubId:999 githubUsername:octocat", identity.Summary())
	assert.Empty(t, (&Identity{}).Summary())

	long := make([]string, 0, 100)
	for i := 0; i < 100; i++ {
		long = append(long, fmt.Sprintf("user-%d@example.org", i))
	}
	summary := (&Identity{Emails: long}).Summary()
	assert.LessOrEqual(t, len(summary), identitySummaryLimit+3, "the audit log line must stay bounded")
	assert.True(t, strings.HasSuffix(summary, "..."))
}

func TestGetMyClasIclaValidity(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	unsigned := icla("sig-3", "user-a", "cla-group-1", "2024-03-01T00:00:00Z", true)
	unsigned.SignatureSigned = false

	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {
				icla("sig-1", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", true),
				icla("sig-2", "user-a", "cla-group-2", "2024-02-01T00:00:00Z", false),
				unsigned,
			},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err)
	require.Len(t, result.Clas, 2, "unsigned records must be excluded")

	byID := map[string]int{}
	for i, row := range result.Clas {
		byID[row.SignatureID] = i
	}
	valid := result.Clas[byID["sig-1"]]
	assert.Equal(t, utils.ClaTypeICLA, valid.ClaType)
	assert.True(t, valid.Valid)
	assert.True(t, valid.PdfAvailable)
	assert.True(t, valid.Approved)
	assert.Equal(t, int64(2), valid.DocumentMajorVersion)

	invalidated := result.Clas[byID["sig-2"]]
	assert.False(t, invalidated.Valid)
	assert.False(t, invalidated.Approved)
	assert.True(t, invalidated.PdfAvailable, "signed PDF remains downloadable for invalidated ICLAs")
}

func TestGetMyClasEclaValidity(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	companies := &fakeCompanies{byID: map[string]*v1Models.Company{
		"company-1": {CompanyID: "company-1", CompanyName: "Good Corp", SigningEntityName: "Good Corp LLC"},
		"company-2": {CompanyID: "company-2", CompanyName: "Sanctioned Corp", IsSanctioned: true},
		"company-3": {CompanyID: "company-3", CompanyName: "No CCLA Corp"},
	}}
	signaturesService := &fakeSignatures{
		cclas: map[string]*v1Models.Signature{
			"cla-group-1|company-1": {SignatureID: "ccla-1"},
		},
		approvedUserIDs: map[string]bool{"user-a": true},
	}
	docusignEraEcla := ecla("sig-1", "company-1", "2024-01-01T00:00:00Z", true)
	docusignEraEcla.SignatureType = utils.SignatureTypeCLA
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {
				docusignEraEcla,
				ecla("sig-2", "company-2", "2024-02-01T00:00:00Z", true),
				ecla("sig-3", "company-3", "2024-03-01T00:00:00Z", true),
				ecla("sig-4", "company-1", "2024-04-01T00:00:00Z", false),
				ecla("sig-5", "company-5", "2024-05-01T00:00:00Z", true),
			},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err)
	require.Len(t, result.Clas, 5)

	byID := map[string]models.MyCla{}
	for _, row := range result.Clas {
		byID[row.SignatureID] = row
	}

	covered := byID["sig-1"]
	assert.Equal(t, utils.ClaTypeECLA, covered.ClaType)
	assert.True(t, covered.Valid)
	assert.False(t, covered.PdfAvailable, "ECLAs have no signed PDF")
	assert.Equal(t, "Good Corp", covered.CompanyName)
	assert.Equal(t, "Good Corp LLC", covered.SigningEntityName)
	assert.Equal(t, "company-1", covered.CompanyID)

	assert.False(t, byID["sig-2"].Valid, "sanctioned company invalidates the ECLA")
	assert.Equal(t, models.MyClaStatusRevoked, byID["sig-2"].Status)
	assert.False(t, byID["sig-3"].Valid, "missing current CCLA invalidates the ECLA")
	assert.Equal(t, models.MyClaStatusUnknown, byID["sig-3"].Status)
	assert.False(t, byID["sig-4"].Valid, "signature_approved=false invalidates the ECLA")
	assert.Equal(t, models.MyClaStatusInvalidated, byID["sig-4"].Status)
	assert.False(t, byID["sig-5"].Valid, "unknown company invalidates the ECLA")
	assert.Equal(t, models.MyClaStatusUnknown, byID["sig-5"].Status)
	assert.Empty(t, byID["sig-5"].CompanyName)
}

func TestGetMyClasEclaNotOnCurrentApprovalList(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	companies := &fakeCompanies{byID: map[string]*v1Models.Company{
		"company-1": {CompanyID: "company-1", CompanyName: "Good Corp"},
	}}
	signaturesService := &fakeSignatures{
		cclas: map[string]*v1Models.Signature{
			"cla-group-1|company-1": {SignatureID: "ccla-1"},
		},
		approvedUserIDs: map[string]bool{},
	}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {ecla("sig-1", "company-1", "2024-01-01T00:00:00Z", true)},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err)
	require.Len(t, result.Clas, 1)
	assert.True(t, result.Clas[0].Approved)
	assert.False(t, result.Clas[0].Valid, "ECLA no longer matching the current approval list is invalid")
	assert.Equal(t, models.MyClaStatusNeedsAttention, result.Clas[0].Status)
}

func TestGetMyClasEclaGitlabGroupFallback(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	companies := &fakeCompanies{byID: map[string]*v1Models.Company{
		"company-1": {CompanyID: "company-1", CompanyName: "GitLab Group Corp"},
	}}
	signaturesService := &fakeSignatures{
		cclas: map[string]*v1Models.Signature{
			"cla-group-1|company-1": {SignatureID: "ccla-1", GitlabOrgApprovalList: []string{"https://gitlab.com/groups/good-group"}},
		},
		approvedUserIDs: map[string]bool{},
	}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {ecla("sig-1", "company-1", "2024-01-01T00:00:00Z", true)},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err)
	require.Len(t, result.Clas, 1)
	assert.True(t, result.Clas[0].Valid, "GitLab-group-approved ECLAs defer to the signature_approved flag")
	assert.Equal(t, models.MyClaStatusUnknown, result.Clas[0].Status, "group membership was never evaluated")
}

func TestGetMyClasEclaApprovalEvaluationError(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	companies := &fakeCompanies{byID: map[string]*v1Models.Company{
		"company-1": {CompanyID: "company-1", CompanyName: "Good Corp"},
	}}
	signaturesService := &fakeSignatures{
		cclas: map[string]*v1Models.Signature{
			"cla-group-1|company-1": {SignatureID: "ccla-1", GitlabOrgApprovalList: []string{"https://gitlab.com/groups/good-group"}},
		},
		userIsApprovedErr: fmt.Errorf("bad approval pattern"),
	}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {ecla("sig-1", "company-1", "2024-01-01T00:00:00Z", true)},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err, "approval-list evaluation problems must not fail the listing")
	require.Len(t, result.Clas, 1)
	assert.False(t, result.Clas[0].Valid, "evaluation errors leave the ECLA not covered - no GitLab fallback")
	assert.Equal(t, models.MyClaStatusUnknown, result.Clas[0].Status, "an evaluation error proves nothing about the approval list")
	assert.Equal(t, models.MyClaStatusReasonUnknown, result.Clas[0].StatusReason)
}

func TestGetMyClasStatus(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	companies := &fakeCompanies{byID: map[string]*v1Models.Company{
		"company-1": {CompanyID: "company-1", CompanyName: "Good Corp"},
	}}
	signaturesService := &fakeSignatures{
		cclas:           map[string]*v1Models.Signature{"cla-group-1|company-1": {SignatureID: "ccla-1"}},
		approvedUserIDs: map[string]bool{"user-a": true},
	}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {
				icla("sig-1", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", true),
				icla("sig-2", "user-a", "cla-group-1", "2024-02-01T00:00:00Z", false),
				ecla("sig-3", "company-1", "2024-03-01T00:00:00Z", true),
				ecla("sig-4", "company-1", "2024-04-01T00:00:00Z", false),
			},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err)
	require.Len(t, result.Clas, 4)
	assert.Equal(t, models.MyClaListSssModeDisabled, result.SssMode, "no screener configured reports disabled")

	byID := map[string]models.MyCla{}
	for _, row := range result.Clas {
		byID[row.SignatureID] = row
	}
	assert.Equal(t, models.MyClaStatusValid, byID["sig-1"].Status)
	assert.Empty(t, byID["sig-1"].StatusReason, "valid rows carry no reason")
	assert.Equal(t, models.MyClaStatusInvalidated, byID["sig-2"].Status)
	assert.Empty(t, byID["sig-2"].StatusReason, "invalidated attributes nothing")
	assert.Equal(t, models.MyClaStatusValid, byID["sig-3"].Status)
	assert.Equal(t, models.MyClaStatusInvalidated, byID["sig-4"].Status)
	assert.Equal(t, models.MyClaFlaggedCheckStored, byID["sig-3"].FlaggedCheck)
	assert.Empty(t, byID["sig-1"].FlaggedCheck, "sanctions are an ECLA concept")
}

func TestGetMyClasStatusNeedsAttentionAndUnknown(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	companies := &fakeCompanies{byID: map[string]*v1Models.Company{
		"company-1": {CompanyID: "company-1", CompanyName: "Good Corp"},
	}}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {ecla("sig-1", "company-1", "2024-01-01T00:00:00Z", true)},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	ccla := map[string]*v1Models.Signature{"cla-group-1|company-1": {SignatureID: "ccla-1"}}

	t.Run("completed approval-list miss", func(t *testing.T) {
		svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{cclas: ccla}, companies, &fakeClaGroups{})
		result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
		require.NoError(t, err)
		require.Len(t, result.Clas, 1)
		assert.Equal(t, models.MyClaStatusNeedsAttention, result.Clas[0].Status)
		assert.Equal(t, models.MyClaStatusReasonNotOnApprovalList, result.Clas[0].StatusReason, "the only reason a Request approval action may gate on")
	})

	t.Run("github organization lookup failed", func(t *testing.T) {
		svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{cclas: ccla, orgLookupFailed: true}, companies, &fakeClaGroups{})
		result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
		require.NoError(t, err)
		require.Len(t, result.Clas, 1)
		assert.Equal(t, models.MyClaStatusUnknown, result.Clas[0].Status, "a failed org lookup must not read as an approval-list miss")
		assert.Equal(t, models.MyClaStatusReasonUnknown, result.Clas[0].StatusReason)
	})
}

func TestGetMyClasDegradesFailedLookups(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}

	t.Run("company lookup failure degrades the row", func(t *testing.T) {
		companies := &fakeCompanies{
			byID:    map[string]*v1Models.Company{"company-1": {CompanyID: "company-1", CompanyName: "Good Corp"}},
			failIDs: map[string]bool{"company-2": true},
		}
		signaturesService := &fakeSignatures{
			cclas:           map[string]*v1Models.Signature{"cla-group-1|company-1": {SignatureID: "ccla-1"}},
			approvedUserIDs: map[string]bool{"user-a": true},
		}
		repo := &fakeRepo{
			byUserID: map[string][]*signatures.ItemSignature{
				"user-a": {
					ecla("sig-1", "company-1", "2024-01-01T00:00:00Z", true),
					ecla("sig-2", "company-2", "2024-02-01T00:00:00Z", true),
					ecla("sig-3", "company-2", "2024-03-01T00:00:00Z", true),
				},
			},
			byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
		}
		svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})

		result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
		require.NoError(t, err, "one unresolvable employer must not fail the whole list")
		require.Len(t, result.Clas, 3)

		byID := map[string]models.MyCla{}
		for _, row := range result.Clas {
			byID[row.SignatureID] = row
		}
		assert.Equal(t, models.MyClaStatusValid, byID["sig-1"].Status, "healthy rows keep their status")
		assert.Equal(t, models.MyClaStatusUnknown, byID["sig-2"].Status)
		assert.Equal(t, models.MyClaStatusUnknown, byID["sig-3"].Status)
		assert.Empty(t, byID["sig-2"].CompanyName)
		assert.False(t, byID["sig-2"].Flagged)
		assert.Equal(t, models.MyClaFlaggedCheckUnavailable, byID["sig-2"].FlaggedCheck, "an unreadable employer cannot be screened, so it is never an absent answer")
		assert.Equal(t, models.MyClaFlaggedCheckStored, byID["sig-1"].FlaggedCheck)
		assert.Equal(t, 2, companies.calls, "a failed employer lookup is cached, not retried per row")
	})

	t.Run("ccla lookup failure degrades the row", func(t *testing.T) {
		companies := &fakeCompanies{byID: map[string]*v1Models.Company{
			"company-1": {CompanyID: "company-1", CompanyName: "Good Corp"},
		}}
		signaturesService := &fakeSignatures{cclaErr: fmt.Errorf("dynamodb unavailable")}
		repo := &fakeRepo{
			byUserID: map[string][]*signatures.ItemSignature{
				"user-a": {
					ecla("sig-1", "company-1", "2024-01-01T00:00:00Z", true),
					ecla("sig-2", "company-1", "2024-02-01T00:00:00Z", true),
				},
			},
			byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
		}
		svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})

		result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
		require.NoError(t, err, "an unresolvable CCLA must not fail the whole list")
		require.Len(t, result.Clas, 2)
		for _, row := range result.Clas {
			assert.Equal(t, models.MyClaStatusUnknown, row.Status)
			assert.False(t, row.Valid)
		}
		assert.Equal(t, 1, signaturesService.cclaCalls, "a failed CCLA lookup is cached, not retried per row")
	})
}

func TestGetMyClasLiveSanctionsScreening(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	companies := &fakeCompanies{byID: map[string]*v1Models.Company{
		"company-1": {CompanyID: "company-1", CompanyName: "Live Flagged Corp"},
		"company-2": {CompanyID: "company-2", CompanyName: "Cleared Corp", IsSanctioned: true, SanctionOrigin: sanctionOriginSSS},
		"company-3": {CompanyID: "company-3", CompanyName: "Unscreenable Corp", IsSanctioned: true},
	}}
	signaturesService := &fakeSignatures{
		cclas: map[string]*v1Models.Signature{
			"cla-group-1|company-1": {SignatureID: "ccla-1"},
			"cla-group-1|company-2": {SignatureID: "ccla-2"},
			"cla-group-1|company-3": {SignatureID: "ccla-3"},
		},
		approvedUserIDs: map[string]bool{"user-a": true},
	}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {
				ecla("sig-1", "company-1", "2024-01-01T00:00:00Z", true),
				ecla("sig-2", "company-2", "2024-02-01T00:00:00Z", true),
				ecla("sig-3", "company-3", "2024-03-01T00:00:00Z", true),
				ecla("sig-4", "company-1", "2024-04-01T00:00:00Z", true),
			},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	screener := &fakeScreener{
		mode:    models.MyClaListSssModeRequired,
		flagged: map[string]bool{"company-1": true, "company-3": true},
		checks:  map[string]string{"company-3": models.MyClaFlaggedCheckUnavailable},
	}
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})
	svc.sanctions = screener

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err, "a screening failure must never fail the listing")
	require.Len(t, result.Clas, 4)
	assert.Equal(t, models.MyClaListSssModeRequired, result.SssMode)

	byID := map[string]models.MyCla{}
	for _, row := range result.Clas {
		byID[row.SignatureID] = row
	}

	liveFlagged := byID["sig-1"]
	assert.True(t, liveFlagged.Flagged, "a live flagged result overrides the stored clean flag")
	assert.Equal(t, models.MyClaFlaggedCheckLive, liveFlagged.FlaggedCheck)
	assert.NotEmpty(t, liveFlagged.FlaggedAt)
	assert.Equal(t, models.MyClaStatusRevoked, liveFlagged.Status)
	assert.Empty(t, liveFlagged.StatusReason)
	assert.False(t, liveFlagged.Valid)

	liveClean := byID["sig-2"]
	assert.False(t, liveClean.Flagged, "a live clean result overrides the stored sanction")
	assert.Equal(t, models.MyClaFlaggedCheckLive, liveClean.FlaggedCheck)
	assert.Empty(t, liveClean.FlaggedAt)
	assert.Equal(t, models.MyClaStatusValid, liveClean.Status)

	unavailable := byID["sig-3"]
	assert.True(t, unavailable.Flagged, "an unusable screen honors the stored flag")
	assert.Equal(t, models.MyClaFlaggedCheckUnavailable, unavailable.FlaggedCheck)
	assert.Equal(t, models.MyClaStatusRevoked, unavailable.Status)

	assert.Equal(t, 1, screener.calls["company-1"], "each distinct employer is screened once per response")
	assert.Equal(t, []sanctionWrite{{companyID: "company-1", sanctioned: true, origin: sanctionOriginSSS}}, companies.writes,
		"only the newly detected sanction is persisted - a live clean and an unusable screen write nothing")
}

func TestGetMyClasPersistsFirstLiveSanction(t *testing.T) {
	const storedDate = "2024-01-15T10:11:12.000000+0000"

	tests := []struct {
		name            string
		company         *v1Models.Company
		writeErr        error
		wantWrites      int
		wantFlaggedAt   string
		wantNoFlaggedAt bool
	}{
		{
			name:       "first live detection is persisted",
			company:    &v1Models.Company{CompanyID: "company-1", CompanyName: "Newly Flagged Corp"},
			wantWrites: 1,
		},
		{
			name:       "an employer flagged again after a clear is restamped",
			company:    &v1Models.Company{CompanyID: "company-1", CompanyName: "Repeat Corp", SanctionedDate: storedDate},
			wantWrites: 1,
		},
		{
			name:          "an already stamped employer is left alone",
			company:       &v1Models.Company{CompanyID: "company-1", CompanyName: "Known Corp", IsSanctioned: true, SanctionOrigin: sanctionOriginSSS, SanctionedDate: storedDate},
			wantWrites:    0,
			wantFlaggedAt: "2024-01-15T10:11:12Z",
		},
		{
			name:            "a failed write reports the flag without a date",
			company:         &v1Models.Company{CompanyID: "company-1", CompanyName: "Unwritable Corp", SanctionedDate: storedDate},
			writeErr:        errors.New("dynamodb unavailable"),
			wantWrites:      0,
			wantNoFlaggedAt: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
			companies := &fakeCompanies{
				byID:     map[string]*v1Models.Company{"company-1": tc.company},
				writeErr: tc.writeErr,
			}
			repo := &fakeRepo{
				byUserID:     map[string][]*signatures.ItemSignature{"user-a": {ecla("sig-1", "company-1", "2024-01-01T00:00:00Z", true)}},
				byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
			}
			signaturesService := &fakeSignatures{
				cclas:           map[string]*v1Models.Signature{"cla-group-1|company-1": {SignatureID: "ccla-1"}},
				approvedUserIDs: map[string]bool{"user-a": true},
			}
			svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})
			svc.sanctions = &fakeScreener{mode: models.MyClaListSssModeRequired, flagged: map[string]bool{"company-1": true}}

			result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
			require.NoError(t, err, "persisting must never fail the listing")
			require.Len(t, result.Clas, 1)
			row := result.Clas[0]

			assert.Len(t, companies.writes, tc.wantWrites)
			if tc.wantWrites > 0 {
				assert.Equal(t, sanctionWrite{companyID: "company-1", sanctioned: true, origin: sanctionOriginSSS}, companies.writes[0],
					"the listing persists through the same SSS-origin write the signing flow uses")
			}
			assert.True(t, row.Flagged)
			assert.Equal(t, models.MyClaFlaggedCheckLive, row.FlaggedCheck)
			if tc.wantNoFlaggedAt {
				assert.Empty(t, row.FlaggedAt, "a flag without a trustworthy date is reported without one")
			} else if tc.wantFlaggedAt != "" {
				assert.Equal(t, tc.wantFlaggedAt, row.FlaggedAt, "the stored date is reported, not the response time")
			} else {
				assert.NotEmpty(t, row.FlaggedAt)
				assert.NotEqual(t, "2024-01-15T10:11:12Z", row.FlaggedAt, "a restamped or unwritten employer reports this observation")
			}
		})
	}
}

// countingScreener records how many screens run at once and holds the first want of them open,
// so the listing can only complete if that many employers are screened concurrently
type countingScreener struct {
	calls    map[string]int
	gate     chan struct{}
	mu       sync.Mutex
	want     int
	arrived  int
	inFlight int
	maxSeen  int
	released bool
}

func (c *countingScreener) Mode() string {
	return models.MyClaListSssModeOptional
}

func (c *countingScreener) peakInFlight() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.maxSeen
}

func (c *countingScreener) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.released {
		c.released = true
		close(c.gate)
	}
}

func (c *countingScreener) ScreenCompany(_ context.Context, company *v1Models.Company) (bool, string) {
	c.mu.Lock()
	c.calls[company.CompanyID]++
	c.arrived++
	c.inFlight++
	if c.inFlight > c.maxSeen {
		c.maxSeen = c.inFlight
	}
	hold := c.arrived <= c.want
	full := c.arrived >= c.want
	c.mu.Unlock()

	if full {
		c.release()
	}
	if hold {
		<-c.gate
	}

	c.mu.Lock()
	c.inFlight--
	c.mu.Unlock()
	return false, models.MyClaFlaggedCheckLive
}

// TestGetMyClasScreensDistinctEmployersInParallel is the 100-ECLAs-over-20-employers case: one
// screen per distinct employer, wantInFlight of them running at once, results merged. It pins
// fetchConcurrency deliberately - the in-flight count is a documented guarantee of the endpoint.
func TestGetMyClasScreensDistinctEmployersInParallel(t *testing.T) {
	const employers, perEmployer, wantInFlight = 20, 5, 8
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	companies := &fakeCompanies{byID: map[string]*v1Models.Company{}}
	cclas := map[string]*v1Models.Signature{}
	var userSigs []*signatures.ItemSignature
	for i := 1; i <= employers; i++ {
		companyID := fmt.Sprintf("company-%d", i)
		companies.byID[companyID] = &v1Models.Company{CompanyID: companyID, CompanyName: companyID}
		cclas["cla-group-1|"+companyID] = &v1Models.Signature{SignatureID: "ccla-" + companyID}
		for j := 1; j <= perEmployer; j++ {
			userSigs = append(userSigs, ecla(fmt.Sprintf("sig-%d-%d", i, j), companyID, "2024-01-01T00:00:00Z", true))
		}
	}
	repo := &fakeRepo{
		byUserID:     map[string][]*signatures.ItemSignature{"user-a": userSigs},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	signaturesService := &fakeSignatures{cclas: cclas, approvedUserIDs: map[string]bool{"user-a": true}}
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})
	screener := &countingScreener{calls: map[string]int{}, gate: make(chan struct{}), want: wantInFlight}
	svc.sanctions = screener

	type outcome struct {
		list *models.MyClaList
		err  error
	}
	done := make(chan outcome, 1)
	go func() {
		list, listErr := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
		done <- outcome{list: list, err: listErr}
	}()

	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.Len(t, result.list.Clas, employers*perEmployer)
	case <-time.After(10 * time.Second):
		screener.release()
		t.Fatalf("the listing stalled with only %d of %d employers screened concurrently", screener.peakInFlight(), wantInFlight)
	}

	assert.Len(t, screener.calls, employers, "every distinct employer is screened")
	for companyID, calls := range screener.calls {
		assert.Equal(t, 1, calls, "employer %s is screened exactly once for all %d of its rows", companyID, perEmployer)
	}
	assert.Equal(t, wantInFlight, screener.peakInFlight(), "the screens run fetchConcurrency at a time")
}

func TestGetMyClaPdfURL(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	unsigned := icla("sig-unsigned", "user-a", "cla-group-1", "2024-03-01T00:00:00Z", true)
	unsigned.SignatureSigned = false
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {
				icla("sig-icla", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", true),
				ecla("sig-ecla", "company-1", "2024-02-01T00:00:00Z", true),
				unsigned,
			},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})
	identity := &Identity{}

	result, err := svc.GetMyClaPdfURL(context.Background(), &Caller{Username: "someone"}, identity, "sig-icla")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "sig-icla", result.SignatureID)
	assert.Equal(t, "https://s3.example.org/contract-group/cla-group-1/icla/user-a/sig-icla.pdf", result.URL)
	assert.Equal(t, int64(900), result.ExpiresInSeconds)

	result, err = svc.GetMyClaPdfURL(context.Background(), &Caller{Username: "someone"}, identity, "sig-ecla")
	require.NoError(t, err)
	assert.Nil(t, result, "ECLAs have no signed PDF")

	result, err = svc.GetMyClaPdfURL(context.Background(), &Caller{Username: "someone"}, identity, "sig-unsigned")
	require.NoError(t, err)
	assert.Nil(t, result, "unsigned records have no signed PDF")

	result, err = svc.GetMyClaPdfURL(context.Background(), &Caller{Username: "someone"}, identity, "sig-of-somebody-else")
	require.NoError(t, err)
	assert.Nil(t, result, "signatures not owned by the resolved identity are not found")

	svc.documentExists = func(_ string) (bool, error) { return false, nil }
	result, err = svc.GetMyClaPdfURL(context.Background(), &Caller{Username: "someone"}, identity, "sig-icla")
	require.NoError(t, err)
	assert.Nil(t, result, "missing S3 objects are reported as not found instead of returning a dead URL")
}

func TestGetMyClaPdfURLOwnershipEnforced(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	victim := &v1Models.User{UserID: "user-v", LfUsername: "victim"}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-v": {icla("sig-victim", "user-v", "cla-group-1", "2024-02-01T00:00:00Z", true)},
		},
		byLFUsername:   map[string][]*v1Models.User{"someone": {userA}, "victim": {victim}},
		byPrimaryEmail: map[string][]*v1Models.User{"victim@example.org": {victim}},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClaPdfURL(context.Background(), &Caller{Username: "someone"}, &Identity{
		LfUsername: "victim",
		Emails:     []string{"victim@example.org"},
	}, "sig-victim")
	require.NoError(t, err)
	assert.Nil(t, result, "a non-admin cannot resolve somebody else's signature")

	result, err = svc.GetMyClaPdfURL(context.Background(), &Caller{Username: "staff-admin", Admin: true}, &Identity{LfUsername: "victim"}, "sig-victim")
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestGetMyIdentities(t *testing.T) {
	profileEmail := "someone@example.org"
	deletedEmail := "old@example.org"
	deleted := true
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone", LfEmail: "Someone@Example.org", Emails: []string{"alt@example.org"}, GithubID: "12345", GithubUsername: "Octocat"}
	userB := &v1Models.User{UserID: "user-b", LfUsername: "someone", GitlabID: "777", GitlabUsername: "octolab"}
	repo := &fakeRepo{byLFUsername: map[string][]*v1Models.User{"someone": {userA, userB}}}
	platform := &fakePlatform{
		user: &platformModels.User{ID: "sfid-1", Username: "someone", Email: &profileEmail, Emails: []*platformModels.Email{{EmailAddress: &deletedEmail, IsDeleted: &deleted}}},
		identities: []*platformModels.UserIdentity{
			{Source: "github", Username: "Octocat", Email: "someone-gh@example.org"},
			{Source: "gerrit", Username: "someone"},
			{Source: "slack", Username: "not-a-code-identity"},
		},
	}
	svc := newTestService(repo, platform, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyIdentities(context.Background(), "someone")
	require.NoError(t, err)
	assert.Equal(t, "someone", result.LfUsername)
	assert.Equal(t, []string{
		"email:alt@example.org",
		"email:someone-gh@example.org",
		"email:someone@example.org",
		"gerrit-username:someone",
		"github-id:12345",
		"github-username:Octocat",
		"gitlab-id:777",
		"gitlab-username:octolab",
		"lf-username:someone",
	}, result.Identities)
	assert.Equal(t, int64(len(result.Identities)), result.ResultCount)

	_, err = svc.GetMyIdentities(context.Background(), "")
	assert.Error(t, err)
}

func TestIdentityIsEmpty(t *testing.T) {
	assert.True(t, (&Identity{}).IsEmpty())
	assert.True(t, (&Identity{LfUsername: " ", Emails: []string{"", "  "}, GerritUsernames: []string{" "}}).IsEmpty())
	assert.False(t, (&Identity{LfUsername: "someone"}).IsEmpty())
	assert.False(t, (&Identity{Emails: []string{"someone@example.org"}}).IsEmpty())
	assert.False(t, (&Identity{SecondaryEmails: []string{"someone@example.org"}}).IsEmpty())
	assert.False(t, (&Identity{GithubIDs: []int64{1}}).IsEmpty())
	assert.False(t, (&Identity{GithubUsernames: []string{"someone"}}).IsEmpty())
	assert.False(t, (&Identity{GitlabIDs: []int64{1}}).IsEmpty())
	assert.False(t, (&Identity{GitlabUsernames: []string{"someone"}}).IsEmpty())
	assert.False(t, (&Identity{GerritUsernames: []string{"someone"}}).IsEmpty())
}
func TestGetMyClasEmitsCompanySanctionedEvent(t *testing.T) {
	const storedDate = "2024-01-15T10:11:12.000000+0000"

	tests := []struct {
		name       string
		company    *v1Models.Company
		writeErr   error
		wantEvents int
	}{
		{
			name:       "a fresh flag is logged",
			company:    &v1Models.Company{CompanyID: "company-1", CompanyName: "Newly Flagged Corp"},
			wantEvents: 1,
		},
		{
			name:       "a re-flag after a clear is logged",
			company:    &v1Models.Company{CompanyID: "company-1", CompanyName: "Repeat Corp", SanctionedDate: storedDate},
			wantEvents: 1,
		},
		{
			name:       "an already sanctioned employer is not re-logged",
			company:    &v1Models.Company{CompanyID: "company-1", CompanyName: "Known Corp", IsSanctioned: true, SanctionedDate: storedDate},
			wantEvents: 0,
		},
		{
			name:       "a date backfill for a known sanction is not logged",
			company:    &v1Models.Company{CompanyID: "company-1", CompanyName: "Dateless Corp", IsSanctioned: true},
			wantEvents: 0,
		},
		{
			name:       "a failed persist is not logged",
			company:    &v1Models.Company{CompanyID: "company-1", CompanyName: "Unwritable Corp"},
			writeErr:   errors.New("dynamodb unavailable"),
			wantEvents: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
			companies := &fakeCompanies{
				byID:     map[string]*v1Models.Company{"company-1": tc.company},
				writeErr: tc.writeErr,
			}
			repo := &fakeRepo{
				byUserID:     map[string][]*signatures.ItemSignature{"user-a": {ecla("sig-1", "company-1", "2024-01-01T00:00:00Z", true)}},
				byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
			}
			signaturesService := &fakeSignatures{
				cclas:           map[string]*v1Models.Signature{"cla-group-1|company-1": {SignatureID: "ccla-1"}},
				approvedUserIDs: map[string]bool{"user-a": true},
			}
			svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})
			svc.sanctions = &fakeScreener{mode: models.MyClaListSssModeRequired, flagged: map[string]bool{"company-1": true}}
			eventsLog := &fakeEvents{}
			svc.eventsService = eventsLog

			_, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
			require.NoError(t, err)

			require.Len(t, eventsLog.logged, tc.wantEvents)
			if tc.wantEvents > 0 {
				logged := eventsLog.logged[0]
				assert.Equal(t, events.CompanySanctioned, logged.EventType)
				assert.Equal(t, "user-a", logged.UserID, "a top-level user identity is required or the events service drops the event")
				assert.Same(t, tc.company, logged.CompanyModel, "the company model is passed so the events service needs no extra lookup")
				assert.Same(t, userA, logged.UserModel, "the listing user whose employer was screened is the event actor")
				_, ok := logged.EventData.(*events.CompanySanctionedEventData)
				assert.True(t, ok)
			}
		})
	}
}

func TestGetMyClasInvalidatedAt(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	invalidated := icla("sig-invalidated", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", false)
	invalidated.DateInvalidated = "2024-03-04T05:06:07.000000+0000"
	invalidated.InvalidatedBy = "admin-user"
	valid := icla("sig-valid", "user-a", "cla-group-1", "2024-02-01T00:00:00Z", true)

	repo := &fakeRepo{
		byUserID:     map[string][]*signatures.ItemSignature{"user-a": {invalidated, valid}},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err)
	byID := map[string]models.MyCla{}
	for _, row := range result.Clas {
		byID[row.SignatureID] = row
	}

	assert.Equal(t, "2024-03-04T05:06:07Z", byID["sig-invalidated"].InvalidatedAt)
	assert.Equal(t, models.MyClaStatusInvalidated, byID["sig-invalidated"].Status)
	assert.Empty(t, byID["sig-valid"].InvalidatedAt)
	assert.Equal(t, models.MyClaStatusValid, byID["sig-valid"].Status)
}
