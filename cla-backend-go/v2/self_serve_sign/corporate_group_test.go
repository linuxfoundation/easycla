// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package self_serve_sign

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/github_organizations"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/cla_groups"
	projectService "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
	v2Sign "github.com/linuxfoundation/easycla/cla-backend-go/v2/sign"
	userService "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const otherCLAGroupID = "62db1b81-6f4a-4b2e-9a4a-0f2d9f0a1b22"

type signingMappings struct {
	projects_cla_groups.Repository
	resolutions []string
	calls       int
	projectSFID string
	foundation  []*projects_cla_groups.ProjectClaGroup
}

func (r *signingMappings) GetClaGroupIDForProject(_ context.Context, projectSFID string) (*projects_cla_groups.ProjectClaGroup, error) {
	index := r.calls
	r.calls++
	if index >= len(r.resolutions) {
		index = len(r.resolutions) - 1
	}
	if r.resolutions[index] == "" {
		return nil, nil
	}
	return &projects_cla_groups.ProjectClaGroup{ProjectSFID: projectSFID, ClaGroupID: r.resolutions[index]}, nil
}

func (r *signingMappings) GetProjectsIdsForFoundation(_ context.Context, _ string) ([]*projects_cla_groups.ProjectClaGroup, error) {
	return r.foundation, nil
}

func (r *signingMappings) GetProjectsIdsForClaGroup(_ context.Context, claGroupID string) ([]*projects_cla_groups.ProjectClaGroup, error) {
	return []*projects_cla_groups.ProjectClaGroup{{ProjectSFID: r.projectSFID, ProjectName: "Covered project", ClaGroupID: claGroupID}}, nil
}

type signingGroups struct {
	cla_groups.Service
	groups map[string]*v1Models.ClaGroup
}

func (r *signingGroups) GetCLAGroupByID(_ context.Context, id string, _ bool) (*v1Models.ClaGroup, error) {
	return r.groups[id], nil
}

func (r *signingGroups) GetCLAGroup(_ context.Context, id string) (*v1Models.ClaGroup, error) {
	return r.groups[id], nil
}

func signingGroup(id string) *v1Models.ClaGroup {
	return &v1Models.ClaGroup{
		ProjectID:          id,
		ProjectName:        "Group " + id,
		ProjectCCLAEnabled: true,
		ProjectCorporateDocuments: []v1Models.ClaGroupDocument{{
			DocumentName:         "Agreement " + id,
			DocumentContentType:  "application/pdf",
			DocumentContent:      "document for " + id,
			DocumentMajorVersion: "1",
			DocumentMinorVersion: "0",
			DocumentCreationDate: "2026-09-01T00:00:00Z",
		}},
	}
}

type signingCompanies struct {
	company.IRepository
	model *v1Models.Company
}

func (r *signingCompanies) GetCompanyByExternalID(_ context.Context, _ string) (*v1Models.Company, error) {
	return r.model, nil
}

func (r *signingCompanies) GetCompany(_ context.Context, _ string) (*v1Models.Company, error) {
	return r.model, nil
}

type signingCompanyService struct {
	company.IService
	aclCalls int
}

func (s *signingCompanyService) AddUserToCompanyAccessList(_ context.Context, _, _ string) error {
	s.aclCalls++
	return nil
}

type signingUsers struct {
	users.Service
}

func (s *signingUsers) GetUserByUserName(_ string, _ bool) (*v1Models.User, error) {
	return &v1Models.User{UserID: "manager-id", Username: "CLA Manager", LfUsername: "manager"}, nil
}

type signingSignatures struct {
	signatures.SignatureService
	queriedGroup string
	saved        []*signatures.ItemSignature
}

func (s *signingSignatures) GetCorporateSignatures(_ context.Context, projectID, _ string, _, _ *bool) ([]*v1Models.Signature, error) {
	s.queriedGroup = projectID
	return nil, nil
}

func (s *signingSignatures) SaveOrUpdateSignature(_ context.Context, signature *signatures.ItemSignature) error {
	s.saved = append(s.saved, signature)
	return nil
}

type signingHTTP struct {
	t                *testing.T
	rootProject      bool
	userLookups      int
	signatoryLookups int
	envelopes        []v2Sign.DocuSignEnvelopeRequest
	tokenReady       chan struct{}
}

type signingTokenBody struct {
	io.Reader
	ready chan struct{}
}

func (b *signingTokenBody) Close() error {
	close(b.ready)
	return nil
}

func (h *signingHTTP) RoundTrip(r *http.Request) (*http.Response, error) {
	response := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Request:    r,
	}
	body := ""
	switch {
	case r.URL.Host == "auth.signing.invalid" && r.URL.Path == "/oauth/token":
		response.Body = &signingTokenBody{
			Reader: strings.NewReader(`{"access_token":"unit-test-token","token_type":"Bearer","expires_in":3600}`),
			ready:  h.tokenReady,
		}
		return response, nil
	case r.URL.Host == "platform.signing.invalid" && strings.HasPrefix(r.URL.Path, "/project-service/v1/projects/"):
		body = `{"id":"` + strings.TrimPrefix(r.URL.Path, "/project-service/v1/projects/") + `"}`
		if !h.rootProject {
			body = strings.TrimSuffix(body, "}") + `,"foundation":{"id":"foundation","name":"Foundation","slug":"foundation"}}`
		}
	case r.URL.Host == "platform.signing.invalid" && r.URL.Path == "/user-service/v1/users":
		h.userLookups++
		if r.URL.Query().Get("email") != "" {
			h.signatoryLookups++
			body = `{"data":[]}`
		} else {
			body = `{"data":[{"username":"manager","name":"CLA Manager","emails":[{"emailAddress":"manager@example.org","isPrimary":true}]}]}`
		}
	case r.URL.Host == "docusign.signing.invalid" && r.URL.Path == "/oauth/token":
		body = `{"access_token":"unit-test-docusign-token"}`
	case r.URL.Host == "docusign.signing.invalid" && r.URL.Path == "/accounts/test-account/envelopes":
		var envelope v2Sign.DocuSignEnvelopeRequest
		require.NoError(h.t, json.NewDecoder(r.Body).Decode(&envelope))
		h.envelopes = append(h.envelopes, envelope)
		response.StatusCode = http.StatusCreated
		body = `{"envelopeId":"test-envelope"}`
	case r.URL.Host == "docusign.signing.invalid" && r.URL.Path == "/accounts/test-account/envelopes/test-envelope/recipients":
		body = `{"signers":[{"clientUserId":"test-signature"}]}`
	case r.URL.Host == "docusign.signing.invalid" && r.URL.Path == "/accounts/test-account/envelopes/test-envelope/views/recipient":
		response.StatusCode = http.StatusCreated
		body = `{"url":"https://docusign.signing.invalid/sign"}`
	default:
		h.t.Errorf("unexpected HTTP request (no network allowed): %s %s", r.Method, r.URL)
		return nil, fmt.Errorf("unexpected HTTP request: %s %s", r.Method, r.URL)
	}
	response.Body = io.NopCloser(strings.NewReader(body))
	return response, nil
}

func setupSigningHTTP(t *testing.T, rootProject bool) *signingHTTP {
	t.Helper()
	transport := &signingHTTP{t: t, rootProject: rootProject, tokenReady: make(chan struct{})}
	oldTransport, oldClient := http.DefaultTransport, http.DefaultClient
	http.DefaultTransport = transport
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() {
		http.DefaultTransport, http.DefaultClient = oldTransport, oldClient
	})
	token.Init("test-client", "test-secret", "https://auth.signing.invalid/oauth/token", "test-audience")
	select {
	case <-transport.tokenReady:
	case <-time.After(5 * time.Second):
		t.Fatal("mock token initialization did not complete")
	}
	projectService.InitClient("https://platform.signing.invalid")
	userService.InitClient("https://platform.signing.invalid", "test-api-key")
	t.Setenv("DOCUSIGN_INTEGRATOR_KEY", "test-integrator")
	t.Setenv("DOCUSIGN_USER_ID", "test-user")
	t.Setenv("DOCUSIGN_AUTH_SERVER", "docusign.signing.invalid")
	t.Setenv("DOCUSIGN_ROOT_URL", "https://docusign.signing.invalid")
	t.Setenv("DOCUSIGN_ACCOUNT_ID", "test-account")
	return transport
}

func TestCorporateSigningCLAGroupBinding(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	privateKey := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))

	testCases := []struct {
		name             string
		selected         string
		resolutions      []string
		root             bool
		rootGroups       []string
		childMappingOnly bool
		missingGroup     bool
		wrongGroupRecord bool
		legacy           bool
		selfSign         bool
		wantGroup        string
		wantErr          error
		wantText         string
		wantResolutions  int
	}{
		{
			name:     "matching selection binds the envelope even if a later resolution would differ",
			selected: testCLAGroupID, resolutions: []string{testCLAGroupID, testCLAGroupID, otherCLAGroupID},
			wantGroup: testCLAGroupID, wantResolutions: 2,
		},
		{
			name:     "compact upper-case selection binds the canonical group",
			selected: strings.ToUpper(strings.ReplaceAll(testCLAGroupID, "-", "")), resolutions: []string{testCLAGroupID},
			wantGroup: testCLAGroupID, wantResolutions: 2,
		},
		{
			name:     "wrapper rejects a different group before entering the engine",
			selected: otherCLAGroupID, resolutions: []string{testCLAGroupID},
			wantErr: v2Sign.ErrCLAGroupMismatch, wantResolutions: 1,
		},
		{
			name:     "engine rejects a mapping changed after the wrapper validated it",
			selected: testCLAGroupID, resolutions: []string{testCLAGroupID, otherCLAGroupID},
			wantErr: v2Sign.ErrCLAGroupMismatch, wantResolutions: 2,
		},
		{
			name:     "engine rejects a mapping removed after the wrapper validated it",
			selected: testCLAGroupID, resolutions: []string{testCLAGroupID, ""},
			wantErr: projects_cla_groups.ErrProjectNotAssociatedWithClaGroup, wantResolutions: 2,
		},
		{
			name:     "engine rejects a nonexistent mapped group",
			selected: testCLAGroupID, resolutions: []string{testCLAGroupID}, missingGroup: true,
			wantErr: projects_cla_groups.ErrCLAGroupDoesNotExist, wantResolutions: 2,
		},
		{
			name:     "engine rejects a loaded group different from the selected group",
			selected: testCLAGroupID, resolutions: []string{testCLAGroupID}, wrongGroupRecord: true,
			wantErr: v2Sign.ErrCLAGroupMismatch, wantResolutions: 2,
		},
		{
			name:     "foundation resolution selects the same group",
			selected: testCLAGroupID, resolutions: []string{testCLAGroupID}, root: true, rootGroups: []string{testCLAGroupID},
			wantGroup: testCLAGroupID, wantResolutions: 1,
		},
		{
			name:     "foundation resolution must match the wrapper's selected group",
			selected: testCLAGroupID, resolutions: []string{testCLAGroupID}, root: true, rootGroups: []string{otherCLAGroupID},
			wantErr: v2Sign.ErrCLAGroupMismatch, wantResolutions: 1,
		},
		{
			name:     "foundation child-only mappings cannot fall through to another group",
			selected: testCLAGroupID, resolutions: []string{testCLAGroupID}, root: true, rootGroups: []string{testCLAGroupID}, childMappingOnly: true,
			wantErr: v2Sign.ErrCLAGroupMismatch, wantResolutions: 1,
		},
		{
			name:     "ambiguous foundation retains its existing rejection",
			selected: testCLAGroupID, resolutions: []string{testCLAGroupID}, root: true, rootGroups: []string{testCLAGroupID, otherCLAGroupID},
			wantText: "multiple cla-groups are associated", wantResolutions: 1,
		},
		{
			name:        "legacy email without an expectation still uses its resolved group",
			resolutions: []string{otherCLAGroupID}, legacy: true,
			wantGroup: otherCLAGroupID, wantResolutions: 1,
		},
		{
			name:        "legacy self-sign without an expectation still works",
			resolutions: []string{otherCLAGroupID}, legacy: true, selfSign: true,
			wantGroup: otherCLAGroupID, wantResolutions: 1,
		},
		{
			name:        "self serve self-sign without a selection remains supported",
			resolutions: []string{testCLAGroupID}, selfSign: true,
			wantGroup: testCLAGroupID, wantResolutions: 2,
		},
	}

	for index, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			transport := setupSigningHTTP(t, tc.root)
			projectSFID := fmt.Sprintf("group-binding-project-%d", index)
			mappings := &signingMappings{resolutions: tc.resolutions, projectSFID: projectSFID}
			for _, groupID := range tc.rootGroups {
				mappedProject := projectSFID
				if tc.childMappingOnly {
					mappedProject = "child-project"
				}
				mappings.foundation = append(mappings.foundation, &projects_cla_groups.ProjectClaGroup{ProjectSFID: mappedProject, ClaGroupID: groupID})
			}
			groups := &signingGroups{groups: map[string]*v1Models.ClaGroup{
				testCLAGroupID: signingGroup(testCLAGroupID), otherCLAGroupID: signingGroup(otherCLAGroupID),
			}}
			if tc.missingGroup {
				delete(groups.groups, testCLAGroupID)
			}
			if tc.wrongGroupRecord {
				groups.groups[testCLAGroupID] = signingGroup(otherCLAGroupID)
			}
			companies, _ := corporateFakes()
			companyRepo := &signingCompanies{model: companies.byExternalID}
			companyService := &signingCompanyService{}
			signatureService := &signingSignatures{}
			signer := v2Sign.NewService("https://cla.signing.invalid", "https://legacy.signing.invalid",
				companyRepo, groups, mappings, companyService, groups, privateKey, &signingUsers{}, signatureService,
				nil, nil, github_organizations.Service{}, nil, "", "", nil, nil, nil, nil, nil, nil, false, false)
			input := corporateInput()
			input.ProjectSfid = &projectSFID
			input.ClaGroupID = tc.selected
			input.SendAsEmail = !tc.selfSign
			input.AuthorityName = "Alex Signatory"
			input.AuthorityEmail = "signatory@example.org"
			input.AuthorityAcked = tc.selfSign
			input.EmbargoAcked = tc.selfSign

			var signatureID string
			var signErr error
			if tc.legacy {
				result, err := signer.RequestCorporateSignature(context.Background(), "manager", "", &models.CorporateSignatureInput{
					ProjectSfid: input.ProjectSfid, CompanySfid: input.CompanySfid, SendAsEmail: input.SendAsEmail,
					AuthorityName: input.AuthorityName, AuthorityEmail: input.AuthorityEmail, ReturnURL: input.ReturnURL,
				})
				signErr = err
				if result != nil {
					signatureID = result.SignatureID
				}
			} else {
				svc := newCorporateTestService(signer, companies, mappings)
				result, err := svc.RequestCorporateSignature(context.Background(), "manager", "", input)
				signErr = err
				if result != nil {
					signatureID = result.SignatureID
					assert.Equal(t, tc.wantGroup, result.ClaGroupID)
				}
			}

			assert.Equal(t, tc.wantResolutions, mappings.calls)
			if tc.wantErr != nil || tc.wantText != "" {
				require.Error(t, signErr)
				if tc.wantErr != nil {
					assert.ErrorIs(t, signErr, tc.wantErr)
				}
				if tc.wantText != "" {
					assert.Contains(t, signErr.Error(), tc.wantText)
				}
				assert.Empty(t, signatureID)
				assert.Empty(t, transport.envelopes)
				assert.Zero(t, transport.userLookups, "no user lookup or signatory-role preparation before the group check")
				assert.Empty(t, signatureService.queriedGroup)
				assert.Empty(t, signatureService.saved)
				assert.Zero(t, companyService.aclCalls)
				return
			}
			require.NoError(t, signErr)
			require.Len(t, signatureService.saved, 1)
			assert.Equal(t, tc.wantGroup, signatureService.queriedGroup)
			assert.Equal(t, tc.wantGroup, signatureService.saved[0].SignatureProjectID)
			assert.Equal(t, signatureID, signatureService.saved[0].SignatureID)
			assert.Equal(t, 1, companyService.aclCalls)
			assert.Equal(t, 1, transport.signatoryLookups)
			require.Len(t, transport.envelopes, 1)
			envelope := transport.envelopes[0]
			assert.Equal(t, "sent", envelope.Status)
			require.Len(t, envelope.Documents, 1)
			assert.Equal(t, "Agreement "+tc.wantGroup, envelope.Documents[0].Name)
			require.Len(t, envelope.Recipients.Signers, 1)
			if !tc.selfSign {
				assert.Equal(t, input.AuthorityName, envelope.Recipients.Signers[0].Name)
				assert.Equal(t, input.AuthorityEmail.String(), envelope.Recipients.Signers[0].Email)
				assert.Empty(t, envelope.Recipients.Signers[0].ClientUserId, "email delivery stays enabled")
			} else {
				assert.Equal(t, signatureID, envelope.Recipients.Signers[0].ClientUserId)
			}
		})
	}
}

func TestBoundCorporateSignerRequiresAGroup(t *testing.T) {
	signer := v2Sign.NewService("", "", nil, nil, nil, nil, nil, "", nil, nil,
		nil, nil, github_organizations.Service{}, nil, "", "", nil, nil, nil, nil, nil, nil, false, false)
	for _, claGroupID := range []string{"", "  "} {
		result, err := signer.RequestCorporateSignatureForCLAGroup(context.Background(), "", "", nil, claGroupID)
		assert.ErrorIs(t, err, v2Sign.ErrCLAGroupRequired)
		assert.Nil(t, result)
	}
}
