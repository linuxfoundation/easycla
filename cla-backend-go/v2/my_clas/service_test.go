// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"fmt"
	"testing"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
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
	return nil, &utils.CompanyNotFound{CompanyID: companyID}
}

type fakeClaGroups struct {
	names map[string]string
}

func (f *fakeClaGroups) GetCLAGroupNameByID(_ context.Context, claGroupID string) (string, error) {
	if name, ok := f.names[claGroupID]; ok {
		return name, nil
	}
	return "", projects_cla_groups.ErrCLAGroupDoesNotExist
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

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{
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

	result, err := svc.GetMyClas(context.Background(), "alice", false, &Identity{GithubIDs: []int64{12345}})
	require.NoError(t, err)
	assert.Empty(t, result.SkippedIdentities, "a numeric ID stored on any of the caller's LFID records is authorized")
	assert.ElementsMatch(t, []string{"user-a1", "user-a2", "user-a3"}, result.UserIds, "all records per key are unioned")
	assert.Equal(t, int64(3), result.ResultCount)

	pdf, err := svc.GetMyClaPdfURL(context.Background(), "alice", false, &Identity{}, "sig-a2")
	require.NoError(t, err)
	require.NotNil(t, pdf, "a PDF owned by the second LFID record is downloadable")
}

func TestGetMyClasNoMatches(t *testing.T) {
	svc := newTestService(&fakeRepo{}, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), "missing", false, &Identity{})
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

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{
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

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{
		GithubUsernames: []string{"octocat"},
		GerritUsernames: []string{"old-ldap-id"},
	})
	require.NoError(t, err)
	assert.Empty(t, result.SkippedIdentities)
	assert.ElementsMatch(t, []string{"user-a", "user-gh", "user-gerrit"}, result.UserIds,
		"the canonical spelling from user-service finds records stored with exact-match keys")
	assert.Equal(t, 1, platform.lookups, "platform identities are loaded once")

	result, err = svc.GetMyClas(context.Background(), "someone", false, &Identity{
		GithubUsernames: []string{"not-a-code-identity"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"githubUsername:not-a-code-identity"}, result.SkippedIdentities,
		"a slack username must not authorize a github search")

	result, err = svc.GetMyClas(context.Background(), "someone", false, &Identity{
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

	result, err := svc.GetMyClas(context.Background(), "staff-admin", true, &Identity{LfUsername: "victim"})
	require.NoError(t, err)
	assert.Empty(t, result.SkippedIdentities)
	assert.Equal(t, "victim", result.LfUsername)
	assert.Equal(t, []string{"user-v"}, result.UserIds)
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

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{})
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

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{})
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
	assert.False(t, byID["sig-3"].Valid, "missing current CCLA invalidates the ECLA")
	assert.False(t, byID["sig-4"].Valid, "signature_approved=false invalidates the ECLA")
	assert.False(t, byID["sig-5"].Valid, "unknown company invalidates the ECLA")
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

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{})
	require.NoError(t, err)
	require.Len(t, result.Clas, 1)
	assert.True(t, result.Clas[0].Approved)
	assert.False(t, result.Clas[0].Valid, "ECLA no longer matching the current approval list is invalid")
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

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{})
	require.NoError(t, err)
	require.Len(t, result.Clas, 1)
	assert.True(t, result.Clas[0].Valid, "GitLab-group-approved ECLAs defer to the signature_approved flag")
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

	result, err := svc.GetMyClas(context.Background(), "someone", false, &Identity{})
	require.NoError(t, err, "approval-list evaluation problems must not fail the listing")
	require.Len(t, result.Clas, 1)
	assert.False(t, result.Clas[0].Valid, "evaluation errors leave the ECLA not covered - no GitLab fallback")
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

	result, err := svc.GetMyClaPdfURL(context.Background(), "someone", false, identity, "sig-icla")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "sig-icla", result.SignatureID)
	assert.Equal(t, "https://s3.example.org/contract-group/cla-group-1/icla/user-a/sig-icla.pdf", result.URL)
	assert.Equal(t, int64(900), result.ExpiresInSeconds)

	result, err = svc.GetMyClaPdfURL(context.Background(), "someone", false, identity, "sig-ecla")
	require.NoError(t, err)
	assert.Nil(t, result, "ECLAs have no signed PDF")

	result, err = svc.GetMyClaPdfURL(context.Background(), "someone", false, identity, "sig-unsigned")
	require.NoError(t, err)
	assert.Nil(t, result, "unsigned records have no signed PDF")

	result, err = svc.GetMyClaPdfURL(context.Background(), "someone", false, identity, "sig-of-somebody-else")
	require.NoError(t, err)
	assert.Nil(t, result, "signatures not owned by the resolved identity are not found")

	svc.documentExists = func(_ string) (bool, error) { return false, nil }
	result, err = svc.GetMyClaPdfURL(context.Background(), "someone", false, identity, "sig-icla")
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

	result, err := svc.GetMyClaPdfURL(context.Background(), "someone", false, &Identity{
		LfUsername: "victim",
		Emails:     []string{"victim@example.org"},
	}, "sig-victim")
	require.NoError(t, err)
	assert.Nil(t, result, "a non-admin cannot resolve somebody else's signature")

	result, err = svc.GetMyClaPdfURL(context.Background(), "staff-admin", true, &Identity{LfUsername: "victim"}, "sig-victim")
	require.NoError(t, err)
	require.NotNil(t, result)
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
