// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"fmt"
	"testing"

	openapiErrors "github.com/go-openapi/errors"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	platformModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
	"github.com/stretchr/testify/assert"
)

type fakeUsers struct {
	byLFUsername      map[string]*v1Models.User
	byEmail           map[string][]*v1Models.User
	bySecondaryEmail  map[string][]*v1Models.User
	byGithubID        map[string]*v1Models.User
	byGithubUsername  map[string]*v1Models.User
	byGitlabID        map[int]*v1Models.User
	byGitlabUsername  map[string]*v1Models.User
	secondaryScanRuns int
}

func (f *fakeUsers) GetUserByLFUserName(lfUserName string) (*v1Models.User, error) {
	return f.byLFUsername[lfUserName], nil
}

func (f *fakeUsers) GetUsersByLFEmail(userEmail string) ([]*v1Models.User, error) {
	if userModels, ok := f.byEmail[userEmail]; ok {
		return userModels, nil
	}
	return nil, &utils.UserNotFound{UserEmail: userEmail}
}

func (f *fakeUsers) GetUsersByEmail(userEmail string) ([]*v1Models.User, error) {
	f.secondaryScanRuns++
	if userModels, ok := f.bySecondaryEmail[userEmail]; ok {
		return userModels, nil
	}
	return nil, nil
}

func (f *fakeUsers) GetUserByGitHubID(gitHubID string) (*v1Models.User, error) {
	if userModel, ok := f.byGithubID[gitHubID]; ok {
		return userModel, nil
	}
	return nil, openapiErrors.NotFound("user not found when searching by user_github_id: %s", gitHubID)
}

func (f *fakeUsers) GetUserByGitHubUsername(gitHubUsername string) (*v1Models.User, error) {
	if userModel, ok := f.byGithubUsername[gitHubUsername]; ok {
		return userModel, nil
	}
	return nil, openapiErrors.NotFound("user not found when searching by user_github_username: %s", gitHubUsername)
}

func (f *fakeUsers) GetUserByGitlabID(gitLabID int) (*v1Models.User, error) {
	if userModel, ok := f.byGitlabID[gitLabID]; ok {
		return userModel, nil
	}
	return nil, openapiErrors.NotFound("user not found when searching by user_gitlab_id: %d", gitLabID)
}

func (f *fakeUsers) GetUserByGitLabUsername(gitLabUsername string) (*v1Models.User, error) {
	if userModel, ok := f.byGitlabUsername[gitLabUsername]; ok {
		return userModel, nil
	}
	return nil, openapiErrors.NotFound("user not found when searching by user_gitlab_username: %s", gitLabUsername)
}

type fakePlatform struct {
	user       *platformModels.User
	identities []*platformModels.UserIdentity
	lookups    int
}

func (f *fakePlatform) GetUserByUsername(_ string) (*platformModels.User, error) {
	f.lookups++
	if f.user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return f.user, nil
}

func (f *fakePlatform) ListUserIdentities(_ string) ([]*platformModels.UserIdentity, error) {
	return f.identities, nil
}

type fakeRepo struct {
	byUserID map[string][]*signatures.ItemSignature
}

func (f *fakeRepo) GetUserCLASignatures(_ context.Context, userID string) ([]*signatures.ItemSignature, error) {
	return f.byUserID[userID], nil
}

type fakeSignatures struct {
	cclas             map[string]*v1Models.Signature
	approvedUserIDs   map[string]bool
	userIsApprovedErr error
}

func (f *fakeSignatures) GetCorporateSignature(_ context.Context, claGroupID, companyID string, _, _ *bool) (*v1Models.Signature, error) {
	return f.cclas[claGroupID+"|"+companyID], nil
}

func (f *fakeSignatures) UserIsApproved(_ context.Context, user *v1Models.User, _ *v1Models.Signature) (bool, error) {
	if f.userIsApprovedErr != nil {
		return false, f.userIsApprovedErr
	}
	return f.approvedUserIDs[user.UserID], nil
}

type fakeCompanies struct {
	byID map[string]*v1Models.Company
}

func (f *fakeCompanies) GetCompany(_ context.Context, companyID string) (*v1Models.Company, error) {
	if companyModel, ok := f.byID[companyID]; ok {
		return companyModel, nil
	}
	return nil, fmt.Errorf("company does not exist: %s", companyID)
}

type fakeClaGroups struct {
	names map[string]string
}

func (f *fakeClaGroups) GetCLAGroupNameByID(_ context.Context, claGroupID string) (string, error) {
	if name, ok := f.names[claGroupID]; ok {
		return name, nil
	}
	return "", fmt.Errorf("cla group does not exist")
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

// ecla builds an auto-created-style ECLA (signature_type=ecla); DocuSign-era ECLAs carry
// signature_type=cla instead - classification must not depend on it, so tests use both.
func ecla(signatureID, userID, claGroupID, companyID, signedOn string, approved bool) *signatures.ItemSignature {
	sig := icla(signatureID, userID, claGroupID, signedOn, approved)
	sig.SignatureUserCompanyID = companyID
	sig.SignatureType = utils.ClaTypeECLA
	return sig
}

func newTestService(repo Repository, usersService UsersService, platform PlatformUsersService, signaturesService SignaturesService, companyRepo CompanyRepository, claGroups ProjectsCLAGroupsRepository) *service {
	svc := NewService(repo, usersService, platform, signaturesService, companyRepo, claGroups).(*service)
	svc.presign = func(filename string) (string, error) {
		return "https://s3.example.org/" + filename, nil
	}
	return svc
}

func TestGetMyClasUnionAndDedupe(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone", LfEmail: "someone@example.org", GithubID: "12345"}
	userB := &v1Models.User{UserID: "user-b", GithubID: "12345"}

	usersService := &fakeUsers{
		byLFUsername: map[string]*v1Models.User{"someone": userA},
		byEmail:      map[string][]*v1Models.User{"someone@example.org": {userA, userB}},
		byGithubID:   map[string]*v1Models.User{"12345": userB},
	}
	repo := &fakeRepo{byUserID: map[string][]*signatures.ItemSignature{
		"user-a": {icla("sig-1", "user-a", "cla-group-1", "2024-01-02T00:00:00Z", true)},
		"user-b": {icla("sig-2", "user-b", "cla-group-1", "2025-01-02T00:00:00Z", true)},
	}}
	svc := newTestService(repo, usersService, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{names: map[string]string{"cla-group-1": "My CLA Group"}})

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{
		Emails:    []string{"Someone@Example.org "},
		GithubIDs: []int64{12345},
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"user-a", "user-b"}, result.UserIds)
	assert.Empty(t, result.SkippedIdentities)
	assert.Equal(t, int64(2), result.ResultCount)
	assert.Len(t, result.Clas, 2)
	// sorted by signedOn descending
	assert.Equal(t, "sig-2", result.Clas[0].SignatureID)
	assert.Equal(t, "sig-1", result.Clas[1].SignatureID)
	assert.Equal(t, "My CLA Group", result.Clas[0].ClaGroupName)
}

func TestGetMyClasNoMatches(t *testing.T) {
	svc := newTestService(&fakeRepo{}, &fakeUsers{}, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), "missing", false, &Identity{})
	assert.NoError(t, err)
	assert.Empty(t, result.UserIds)
	assert.Empty(t, result.Clas)
	assert.Equal(t, int64(0), result.ResultCount)
}

func TestGetMyClasOwnershipRejectsForeignIdentities(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone", LfEmail: "someone@example.org", GithubID: "12345", GithubUsername: "someone-gh"}
	victim := &v1Models.User{UserID: "user-v", LfUsername: "victim"}

	usersService := &fakeUsers{
		byLFUsername:     map[string]*v1Models.User{"someone": userA, "victim": victim},
		byEmail:          map[string][]*v1Models.User{"victim@example.org": {victim}},
		byGithubID:       map[string]*v1Models.User{"999": victim},
		byGithubUsername: map[string]*v1Models.User{"victim-gh": victim},
		byGitlabID:       map[int]*v1Models.User{7: victim},
		byGitlabUsername: map[string]*v1Models.User{"victim-gl": victim},
	}
	repo := &fakeRepo{byUserID: map[string][]*signatures.ItemSignature{
		"user-a": {icla("sig-own", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", true)},
		"user-v": {icla("sig-victim", "user-v", "cla-group-1", "2024-02-01T00:00:00Z", true)},
	}}
	svc := newTestService(repo, usersService, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{
		LfUsername:      "victim",
		Emails:          []string{"victim@example.org"},
		SecondaryEmails: []string{"victim-alt@example.org"},
		GithubIDs:       []int64{999},
		GithubUsernames: []string{"victim-gh"},
		GitlabIDs:       []int64{7},
		GitlabUsernames: []string{"victim-gl"},
		GerritUsernames: []string{"victim"},
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"user-a"}, result.UserIds, "only the caller's own record is searched")
	assert.Len(t, result.Clas, 1)
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
}

func TestGetMyClasOwnershipViaEasyCLARecord(t *testing.T) {
	userA := &v1Models.User{
		UserID: "user-a", LfUsername: "someone", LfEmail: "someone@example.org",
		Emails: []string{"alt@example.org"}, GithubID: "12345", GithubUsername: "someone-gh",
		GitlabID: "777", GitlabUsername: "someone-gl",
	}
	userB := &v1Models.User{UserID: "user-b"}
	userC := &v1Models.User{UserID: "user-c"}

	usersService := &fakeUsers{
		byLFUsername:     map[string]*v1Models.User{"someone": userA},
		bySecondaryEmail: map[string][]*v1Models.User{"alt@example.org": {userB}},
		byGitlabID:       map[int]*v1Models.User{777: userC},
	}
	repo := &fakeRepo{byUserID: map[string][]*signatures.ItemSignature{
		"user-b": {icla("sig-b", "user-b", "cla-group-1", "2024-01-01T00:00:00Z", true)},
		"user-c": {icla("sig-c", "user-c", "cla-group-1", "2024-02-01T00:00:00Z", true)},
	}}
	platform := &fakePlatform{}
	svc := newTestService(repo, usersService, platform, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{
		SecondaryEmails: []string{"Alt@Example.org"},
		GitlabIDs:       []int64{777},
	})
	assert.NoError(t, err)
	assert.Empty(t, result.SkippedIdentities)
	assert.ElementsMatch(t, []string{"user-a", "user-b", "user-c"}, result.UserIds)
	assert.Equal(t, 1, usersService.secondaryScanRuns, "secondary-email scan runs only for the provided value")
	assert.Equal(t, 0, platform.lookups, "user-service is not consulted when the EasyCLA record covers the identities")
}

func TestGetMyClasOwnershipViaPlatformIdentities(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	userGh := &v1Models.User{UserID: "user-gh"}
	userGerrit := &v1Models.User{UserID: "user-gerrit"}

	usersService := &fakeUsers{
		byLFUsername:     map[string]*v1Models.User{"someone": userA, "old-ldap-id": userGerrit},
		byGithubUsername: map[string]*v1Models.User{"octocat": userGh},
	}
	repo := &fakeRepo{byUserID: map[string][]*signatures.ItemSignature{
		"user-gh":     {icla("sig-gh", "user-gh", "cla-group-1", "2024-01-01T00:00:00Z", true)},
		"user-gerrit": {icla("sig-gerrit", "user-gerrit", "cla-group-1", "2024-02-01T00:00:00Z", true)},
	}}
	platform := &fakePlatform{
		user: &platformModels.User{ID: "sfid-1", Username: "someone"},
		identities: []*platformModels.UserIdentity{
			{Source: "github", Username: "Octocat", Email: "someone-gh@example.org"},
			{Source: "gerrit", Username: "old-ldap-id"},
			{Source: "slack", Username: "not-a-code-identity"},
		},
	}
	svc := newTestService(repo, usersService, platform, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{
		GithubUsernames: []string{"octocat"},
		GerritUsernames: []string{"old-ldap-id"},
	})
	assert.NoError(t, err)
	assert.Empty(t, result.SkippedIdentities)
	assert.ElementsMatch(t, []string{"user-a", "user-gh", "user-gerrit"}, result.UserIds)
	assert.Equal(t, 1, platform.lookups, "platform identities are loaded once")

	// a slack username must not authorize a github/gitlab/gerrit search
	result, err = svc.GetMyClas(context.Background(), "someone", false, &Identity{
		GithubUsernames: []string{"not-a-code-identity"},
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"githubUsername:not-a-code-identity"}, result.SkippedIdentities)
}

func TestGetMyClasAdminBypass(t *testing.T) {
	victim := &v1Models.User{UserID: "user-v", LfUsername: "victim"}
	usersService := &fakeUsers{
		byLFUsername: map[string]*v1Models.User{"victim": victim},
	}
	repo := &fakeRepo{byUserID: map[string][]*signatures.ItemSignature{
		"user-v": {icla("sig-victim", "user-v", "cla-group-1", "2024-02-01T00:00:00Z", true)},
	}}
	svc := newTestService(repo, usersService, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), "staff-admin", true, &Identity{LfUsername: "victim"})
	assert.NoError(t, err)
	assert.Empty(t, result.SkippedIdentities)
	assert.Equal(t, "victim", result.LfUsername)
	assert.Equal(t, []string{"user-v"}, result.UserIds)
}

func TestGetMyClasIclaValidity(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	unsigned := icla("sig-3", "user-a", "cla-group-1", "2024-03-01T00:00:00Z", true)
	unsigned.SignatureSigned = false

	usersService := &fakeUsers{byLFUsername: map[string]*v1Models.User{"someone": userA}}
	repo := &fakeRepo{byUserID: map[string][]*signatures.ItemSignature{
		"user-a": {
			icla("sig-1", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", true),
			icla("sig-2", "user-a", "cla-group-2", "2024-02-01T00:00:00Z", false),
			unsigned,
		},
	}}
	svc := newTestService(repo, usersService, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{})
	assert.NoError(t, err)
	assert.Len(t, result.Clas, 2, "unsigned records must be excluded")

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
	usersService := &fakeUsers{byLFUsername: map[string]*v1Models.User{"someone": userA}}

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
	docusignEraEcla := ecla("sig-1", "user-a", "cla-group-1", "company-1", "2024-01-01T00:00:00Z", true)
	docusignEraEcla.SignatureType = utils.SignatureTypeCLA
	repo := &fakeRepo{byUserID: map[string][]*signatures.ItemSignature{
		"user-a": {
			docusignEraEcla,
			ecla("sig-2", "user-a", "cla-group-1", "company-2", "2024-02-01T00:00:00Z", true),
			ecla("sig-3", "user-a", "cla-group-1", "company-3", "2024-03-01T00:00:00Z", true),
			ecla("sig-4", "user-a", "cla-group-1", "company-1", "2024-04-01T00:00:00Z", false),
		},
	}}
	svc := newTestService(repo, usersService, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{})
	assert.NoError(t, err)
	assert.Len(t, result.Clas, 4)

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
	assert.False(t, byID["sig-3"].Valid, "missing current CCLA invalidates the ECLA")
	assert.False(t, byID["sig-4"].Valid, "signature_approved=false invalidates the ECLA")
}

func TestGetMyClasEclaNotOnCurrentApprovalList(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	usersService := &fakeUsers{byLFUsername: map[string]*v1Models.User{"someone": userA}}
	companies := &fakeCompanies{byID: map[string]*v1Models.Company{
		"company-1": {CompanyID: "company-1", CompanyName: "Good Corp"},
	}}
	signaturesService := &fakeSignatures{
		cclas: map[string]*v1Models.Signature{
			"cla-group-1|company-1": {SignatureID: "ccla-1"},
		},
		approvedUserIDs: map[string]bool{},
	}
	repo := &fakeRepo{byUserID: map[string][]*signatures.ItemSignature{
		"user-a": {ecla("sig-1", "user-a", "cla-group-1", "company-1", "2024-01-01T00:00:00Z", true)},
	}}
	svc := newTestService(repo, usersService, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})
	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{})
	assert.NoError(t, err)
	assert.Len(t, result.Clas, 1)
	assert.True(t, result.Clas[0].Approved)
	assert.False(t, result.Clas[0].Valid, "ECLA no longer matching the current approval list is invalid")
}

func TestGetMyClaPdfURL(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	usersService := &fakeUsers{byLFUsername: map[string]*v1Models.User{"someone": userA}}
	unsigned := icla("sig-unsigned", "user-a", "cla-group-1", "2024-03-01T00:00:00Z", true)
	unsigned.SignatureSigned = false
	repo := &fakeRepo{byUserID: map[string][]*signatures.ItemSignature{
		"user-a": {
			icla("sig-icla", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", true),
			ecla("sig-ecla", "user-a", "cla-group-1", "company-1", "2024-02-01T00:00:00Z", true),
			unsigned,
		},
	}}
	svc := newTestService(repo, usersService, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClaPdfURL(context.Background(), "someone", false, &Identity{}, "sig-icla")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "sig-icla", result.SignatureID)
	assert.Equal(t, "https://s3.example.org/contract-group/cla-group-1/icla/user-a/sig-icla.pdf", result.URL)
	assert.Equal(t, int64(900), result.ExpiresInSeconds)

	result, err = svc.GetMyClaPdfURL(context.Background(), "someone", false, &Identity{}, "sig-ecla")
	assert.NoError(t, err)
	assert.Nil(t, result, "ECLAs have no signed PDF")

	result, err = svc.GetMyClaPdfURL(context.Background(), "someone", false, &Identity{}, "sig-unsigned")
	assert.NoError(t, err)
	assert.Nil(t, result, "unsigned records have no signed PDF")

	result, err = svc.GetMyClaPdfURL(context.Background(), "someone", false, &Identity{}, "sig-of-somebody-else")
	assert.NoError(t, err)
	assert.Nil(t, result, "signatures not owned by the resolved identity are not found")
}

func TestGetMyClaPdfURLOwnershipEnforced(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	victim := &v1Models.User{UserID: "user-v", LfUsername: "victim"}
	usersService := &fakeUsers{
		byLFUsername: map[string]*v1Models.User{"someone": userA, "victim": victim},
		byEmail:      map[string][]*v1Models.User{"victim@example.org": {victim}},
	}
	repo := &fakeRepo{byUserID: map[string][]*signatures.ItemSignature{
		"user-v": {icla("sig-victim", "user-v", "cla-group-1", "2024-02-01T00:00:00Z", true)},
	}}
	svc := newTestService(repo, usersService, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	// a non-admin cannot resolve somebody else's signature, even when passing that
	// person's identity keys explicitly - they are skipped, so the PDF is not found
	result, err := svc.GetMyClaPdfURL(context.Background(), "someone", false, &Identity{
		LfUsername: "victim",
		Emails:     []string{"victim@example.org"},
	}, "sig-victim")
	assert.NoError(t, err)
	assert.Nil(t, result)

	// an admin can
	result, err = svc.GetMyClaPdfURL(context.Background(), "staff-admin", true, &Identity{LfUsername: "victim"}, "sig-victim")
	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestIdentityIsEmpty(t *testing.T) {
	assert.True(t, (&Identity{}).IsEmpty())
	assert.False(t, (&Identity{LfUsername: "someone"}).IsEmpty())
	assert.False(t, (&Identity{Emails: []string{"someone@example.org"}}).IsEmpty())
	assert.False(t, (&Identity{SecondaryEmails: []string{"someone@example.org"}}).IsEmpty())
	assert.False(t, (&Identity{GithubIDs: []int64{1}}).IsEmpty())
	assert.False(t, (&Identity{GithubUsernames: []string{"someone"}}).IsEmpty())
	assert.False(t, (&Identity{GitlabIDs: []int64{1}}).IsEmpty())
	assert.False(t, (&Identity{GitlabUsernames: []string{"someone"}}).IsEmpty())
	assert.False(t, (&Identity{GerritUsernames: []string{"someone"}}).IsEmpty())
}

func TestIsNotFound(t *testing.T) {
	assert.False(t, isNotFound(nil))
	assert.True(t, isNotFound(&utils.UserNotFound{UserEmail: "someone@example.org"}))
	assert.True(t, isNotFound(openapiErrors.NotFound("user not found when searching by user_github_id: %s", "12345")))
	assert.False(t, isNotFound(fmt.Errorf("connection refused")))
}
