// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/go-openapi/strfmt"
	v1ClaManager "github.com/linuxfoundation/easycla/cla-backend-go/cla_manager"
	"github.com/linuxfoundation/easycla/cla-backend-go/emails"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	sigAPI "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	service2 "github.com/linuxfoundation/easycla/cla-backend-go/project/service"
	v1Signatures "github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/stretchr/testify/assert"
)

type fakeManagerService struct {
	v1ClaManager.IService
	requestList  *v1Models.ClaManagerRequestList
	listErr      error
	request      *v1Models.ClaManagerRequest
	requestErr   error
	approved     *v1Models.ClaManagerRequest
	approveErr   error
	denied       *v1Models.ClaManagerRequest
	denyErr      error
	listCalls    [][]string
	getCalls     []string
	approveCalls [][]string
	denyCalls    [][]string
}

func (f *fakeManagerService) GetRequests(companyID, claGroupID string) (*v1Models.ClaManagerRequestList, error) {
	f.listCalls = append(f.listCalls, []string{companyID, claGroupID})
	return f.requestList, f.listErr
}

func (f *fakeManagerService) GetRequest(requestID string) (*v1Models.ClaManagerRequest, error) {
	f.getCalls = append(f.getCalls, requestID)
	return f.request, f.requestErr
}

func (f *fakeManagerService) ApproveRequest(companyID, claGroupID, requestID string) (*v1Models.ClaManagerRequest, error) {
	f.approveCalls = append(f.approveCalls, []string{companyID, claGroupID, requestID})
	return f.approved, f.approveErr
}

func (f *fakeManagerService) DenyRequest(companyID, claGroupID, requestID string) (*v1Models.ClaManagerRequest, error) {
	f.denyCalls = append(f.denyCalls, []string{companyID, claGroupID, requestID})
	return f.denied, f.denyErr
}

type fakeProjectService struct {
	service2.Service
	claGroup *v1Models.ClaGroup
	err      error
}

func (f *fakeProjectService) GetCLAGroupByID(ctx context.Context, claGroupID string) (*v1Models.ClaGroup, error) {
	return f.claGroup, f.err
}

type fakeSignatureService struct {
	v1Signatures.SignatureService
	signatures *v1Models.Signatures
	err        error
	addCalls   [][]string
	addErr     error
}

func (f *fakeSignatureService) GetProjectCompanySignatures(ctx context.Context, params sigAPI.GetProjectCompanySignaturesParams) (*v1Models.Signatures, error) {
	return f.signatures, f.err
}

func (f *fakeSignatureService) AddCLAManager(ctx context.Context, signatureID, claManagerID string) (*v1Models.Signature, error) {
	f.addCalls = append(f.addCalls, []string{signatureID, claManagerID})
	if f.addErr != nil {
		return nil, f.addErr
	}
	return &v1Models.Signature{SignatureID: signatureID}, nil
}

type fakeEventsService struct {
	events.Service
	logged []*events.LogEventArgs
}

func (f *fakeEventsService) LogEventWithContext(ctx context.Context, args *events.LogEventArgs) {
	f.logged = append(f.logged, args)
}

type fakeEmailTemplateService struct {
	renderCalls []string
}

func (f *fakeEmailTemplateService) PrefillV2CLAProjectParams(projectSFIDs []string) ([]emails.CLAProjectParams, error) {
	return nil, nil
}

func (f *fakeEmailTemplateService) GetCLAGroupTemplateParamsFromProjectSFID(claGroupVersion, projectSFID string) (emails.CLAGroupTemplateParams, error) {
	f.renderCalls = append(f.renderCalls, projectSFID)
	return emails.CLAGroupTemplateParams{}, nil
}

func (f *fakeEmailTemplateService) GetCLAGroupTemplateParamsFromCLAGroup(claGroupID string) (emails.CLAGroupTemplateParams, error) {
	return emails.CLAGroupTemplateParams{}, nil
}

type capturedEmail struct {
	subject    string
	body       string
	recipients []string
}

type capturingEmailSender struct {
	sent []capturedEmail
}

func (c *capturingEmailSender) SendEmail(subject string, body string, recipients []string) error {
	c.sent = append(c.sent, capturedEmail{subject: subject, body: body, recipients: recipients})
	return nil
}

func installEmailSender(t *testing.T) *capturingEmailSender {
	t.Helper()
	sender := &capturingEmailSender{}
	prev := utils.GetEmailSender()
	utils.SetEmailSender(sender)
	t.Cleanup(func() { utils.SetEmailSender(prev) })
	return sender
}

func acmeCompany() *v1Models.Company {
	return &v1Models.Company{CompanyID: "company-1", CompanyName: "Acme", CompanyExternalID: "comp-sfid"}
}

func pendingRequest() *v1Models.ClaManagerRequest {
	return &v1Models.ClaManagerRequest{
		RequestID: "req-1",
		CompanyID: "company-1",
		// the shared v1 read projection drops both external ids
		CompanyExternalID: "",
		CompanyName:       "Acme",
		ProjectID:         "cla-group-1",
		ProjectExternalID: "proj-sfid",
		ProjectName:       "My Project",
		UserID:            "user-9",
		UserExternalID:    "",
		UserName:          "Requester",
		UserEmail:         "requester@example.com",
		Status:            "pending",
		Created:           "2026-01-01T00:00:00Z",
		Updated:           "2026-01-02T00:00:00Z",
	}
}

func ccalSignatures() *v1Models.Signatures {
	return &v1Models.Signatures{
		Signatures: []*v1Models.Signature{
			{
				SignatureID: "sig-1",
				SignatureACL: []v1Models.User{
					{Username: "mgr1", LfEmail: strfmt.Email("m1@example.com")},
					{Username: "mgr2", LfEmail: strfmt.Email("m2@example.com")},
				},
			},
		},
	}
}

func TestGetCLAManagerRequests(t *testing.T) {
	t.Run("converts the v1 list and preserves every id field", func(t *testing.T) {
		mgr := &fakeManagerService{requestList: &v1Models.ClaManagerRequestList{Requests: []v1Models.ClaManagerRequest{*pendingRequest()}}}
		s := &service{managerService: mgr}

		result, err := s.GetCLAManagerRequests(context.Background(), acmeCompany(), "cla-group-1")
		assert.Nil(t, err)
		assert.Equal(t, [][]string{{"company-1", "cla-group-1"}}, mgr.listCalls)
		if assert.Len(t, result.Requests, 1) {
			got := result.Requests[0]
			assert.Equal(t, "req-1", got.RequestID)
			assert.Equal(t, "company-1", got.CompanyID)
			assert.Equal(t, "comp-sfid", got.CompanyExternalID, "enriched from the company model - the v1 read projection drops it")
			assert.Equal(t, "Acme", got.CompanyName)
			assert.Equal(t, "cla-group-1", got.ProjectID)
			assert.Equal(t, "proj-sfid", got.ProjectExternalID)
			assert.Equal(t, "My Project", got.ProjectName)
			assert.Equal(t, "user-9", got.UserID)
			assert.Empty(t, got.UserExternalID, "not projected by the legacy read path")
			assert.Equal(t, "Requester", got.UserName)
			assert.Equal(t, "requester@example.com", got.UserEmail)
			assert.Equal(t, "pending", got.Status)
			assert.Equal(t, "2026-01-01T00:00:00Z", got.Created)
			assert.Equal(t, "2026-01-02T00:00:00Z", got.Updated)
		}
	})

	t.Run("empty list marshals as [] not null", func(t *testing.T) {
		mgr := &fakeManagerService{requestList: &v1Models.ClaManagerRequestList{}}
		s := &service{managerService: mgr}

		result, err := s.GetCLAManagerRequests(context.Background(), acmeCompany(), "cla-group-1")
		assert.Nil(t, err)
		body, marshalErr := json.Marshal(result)
		assert.Nil(t, marshalErr)
		assert.JSONEq(t, `{"requests":[]}`, string(body))
	})

	t.Run("propagates the v1 service error", func(t *testing.T) {
		mgr := &fakeManagerService{listErr: errors.New("dynamo down")}
		s := &service{managerService: mgr}

		result, err := s.GetCLAManagerRequests(context.Background(), acmeCompany(), "cla-group-1")
		assert.Nil(t, result)
		assert.EqualError(t, err, "dynamo down")
	})
}

func TestGetCLAManagerRequest(t *testing.T) {
	cases := []struct {
		name      string
		request   *v1Models.ClaManagerRequest
		err       error
		companyID string
		claGroup  string
		wantErr   error
	}{
		{name: "found and matches tenant", request: pendingRequest(), companyID: "company-1", claGroup: "cla-group-1"},
		{name: "missing request maps to not found", request: nil, companyID: "company-1", claGroup: "cla-group-1", wantErr: errRequestNotFound},
		{name: "other company request maps to not found", request: pendingRequest(), companyID: "company-2", claGroup: "cla-group-1", wantErr: errRequestNotFound},
		{name: "other cla group request maps to not found", request: pendingRequest(), companyID: "company-1", claGroup: "cla-group-2", wantErr: errRequestNotFound},
		{name: "lookup error is propagated", err: errors.New("dynamo down"), companyID: "company-1", claGroup: "cla-group-1", wantErr: errors.New("dynamo down")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mgr := &fakeManagerService{request: tc.request, requestErr: tc.err}
			s := &service{managerService: mgr}

			companyModel := acmeCompany()
			companyModel.CompanyID = tc.companyID
			result, err := s.GetCLAManagerRequest(context.Background(), companyModel, tc.claGroup, "req-1")
			assert.Equal(t, []string{"req-1"}, mgr.getCalls)
			if tc.wantErr != nil {
				assert.Nil(t, result)
				assert.EqualError(t, err, tc.wantErr.Error())
				return
			}
			assert.Nil(t, err)
			assert.Equal(t, "req-1", result.RequestID)
			assert.Equal(t, "user-9", result.UserID)
			assert.Equal(t, "comp-sfid", result.CompanyExternalID, "enriched from the company model - the v1 read projection drops it")
		})
	}
}

func TestApproveCLAManagerRequest(t *testing.T) {
	authUser := &auth.User{UserName: "manager-user", Email: "manager@example.com"}
	companyModel := acmeCompany()

	t.Run("approve flips status, updates the signature ACL and notifies managers and requester", func(t *testing.T) {
		approvedRequest := pendingRequest()
		approvedRequest.Status = "approved"
		mgr := &fakeManagerService{request: pendingRequest(), approved: approvedRequest}
		sigs := &fakeSignatureService{signatures: ccalSignatures()}
		ev := &fakeEventsService{}
		emailSvc := &fakeEmailTemplateService{}
		s := &service{
			managerService:       mgr,
			projectService:       &fakeProjectService{claGroup: &v1Models.ClaGroup{ProjectName: "My Project", Version: "v2", ProjectExternalID: "proj-sfid"}},
			signatureService:     sigs,
			eventService:         ev,
			emailTemplateService: emailSvc,
		}

		sender := installEmailSender(t)
		result, err := s.ApproveCLAManagerRequest(context.Background(), authUser, companyModel, "cla-group-1", "req-1")
		assert.Nil(t, err)
		assert.Equal(t, "approved", result.Status)
		assert.Equal(t, "req-1", result.RequestID)
		assert.Equal(t, "comp-sfid", result.CompanyExternalID, "enriched from the company model - the v1 read projection drops it")
		assert.Equal(t, [][]string{{"company-1", "cla-group-1", "req-1"}}, mgr.approveCalls)
		assert.Equal(t, [][]string{{"sig-1", "user-9"}}, sigs.addCalls, "the requester is added to the CCLA signature ACL")

		if assert.Len(t, ev.logged, 1) {
			logged := ev.logged[0]
			assert.Equal(t, events.ClaManagerAccessRequestApproved, logged.EventType)
			assert.Equal(t, "cla-group-1", logged.ProjectID)
			assert.Equal(t, "company-1", logged.CompanyID)
			assert.Equal(t, "manager-user", logged.LfUsername)
			eventData, ok := logged.EventData.(*events.CLAManagerRequestApprovedEventData)
			if assert.True(t, ok) {
				assert.Equal(t, "req-1", eventData.RequestID)
				assert.Equal(t, "Acme", eventData.CompanyName)
				assert.Equal(t, "My Project", eventData.ProjectName)
				assert.Equal(t, "Requester", eventData.UserName)
				assert.Equal(t, "requester@example.com", eventData.UserEmail)
				assert.Equal(t, "manager-user", eventData.ManagerName)
				assert.Equal(t, "manager@example.com", eventData.ManagerEmail)
			}
		}

		assert.Len(t, emailSvc.renderCalls, 3, "one email per existing CLA manager plus one to the requester")
		if assert.Len(t, sender.sent, 3) {
			assert.Equal(t, []string{"m1@example.com"}, sender.sent[0].recipients)
			assert.Equal(t, []string{"m2@example.com"}, sender.sent[1].recipients)
			assert.Equal(t, []string{"requester@example.com"}, sender.sent[2].recipients)
			assert.Contains(t, sender.sent[0].subject, "CLA Manager Access Approval Notice for My Project")
			assert.Contains(t, sender.sent[2].subject, "New CLA Manager Access Approved for My Project")
			assert.Contains(t, sender.sent[0].body, "Requester (requester@example.com)")
		}
	})

	t.Run("missing or cross-tenant request maps to not found before any mutation", func(t *testing.T) {
		otherCompany := pendingRequest()
		otherCompany.CompanyID = "company-other"
		for name, request := range map[string]*v1Models.ClaManagerRequest{"missing": nil, "other company": otherCompany} {
			mgr := &fakeManagerService{request: request}
			s := &service{managerService: mgr}

			result, err := s.ApproveCLAManagerRequest(context.Background(), authUser, companyModel, "cla-group-1", "req-1")
			assert.Nil(t, result, name)
			assert.ErrorIs(t, err, errRequestNotFound, name)
			assert.Empty(t, mgr.approveCalls, name)
		}
	})

	t.Run("no signed CCLA blocks the approval", func(t *testing.T) {
		mgr := &fakeManagerService{request: pendingRequest()}
		s := &service{
			managerService:   mgr,
			projectService:   &fakeProjectService{claGroup: &v1Models.ClaGroup{ProjectName: "My Project"}},
			signatureService: &fakeSignatureService{signatures: &v1Models.Signatures{}},
		}

		result, err := s.ApproveCLAManagerRequest(context.Background(), authUser, companyModel, "cla-group-1", "req-1")
		assert.Nil(t, result)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "error reading CCLA Signatures")
		}
		assert.Empty(t, mgr.approveCalls)
	})

	t.Run("approve error skips the ACL update", func(t *testing.T) {
		mgr := &fakeManagerService{request: pendingRequest(), approveErr: errors.New("status update failed")}
		sigs := &fakeSignatureService{signatures: ccalSignatures()}
		s := &service{
			managerService:   mgr,
			projectService:   &fakeProjectService{claGroup: &v1Models.ClaGroup{ProjectName: "My Project"}},
			signatureService: sigs,
		}

		result, err := s.ApproveCLAManagerRequest(context.Background(), authUser, companyModel, "cla-group-1", "req-1")
		assert.Nil(t, result)
		assert.EqualError(t, err, "status update failed")
		assert.Empty(t, sigs.addCalls)
	})
}

func TestDenyCLAManagerRequest(t *testing.T) {
	authUser := &auth.User{UserName: "manager-user", Email: "manager@example.com"}
	companyModel := acmeCompany()

	t.Run("deny flips status and notifies without touching the signature ACL", func(t *testing.T) {
		deniedRequest := pendingRequest()
		deniedRequest.Status = "denied"
		mgr := &fakeManagerService{request: pendingRequest(), denied: deniedRequest}
		sigs := &fakeSignatureService{signatures: ccalSignatures()}
		ev := &fakeEventsService{}
		emailSvc := &fakeEmailTemplateService{}
		s := &service{
			managerService:       mgr,
			projectService:       &fakeProjectService{claGroup: &v1Models.ClaGroup{ProjectName: "My Project", Version: "v2", ProjectExternalID: "proj-sfid"}},
			signatureService:     sigs,
			eventService:         ev,
			emailTemplateService: emailSvc,
		}

		sender := installEmailSender(t)
		result, err := s.DenyCLAManagerRequest(context.Background(), authUser, companyModel, "cla-group-1", "req-1")
		assert.Nil(t, err)
		assert.Equal(t, "denied", result.Status)
		assert.Equal(t, "comp-sfid", result.CompanyExternalID, "enriched from the company model - the v1 read projection drops it")
		assert.Equal(t, [][]string{{"company-1", "cla-group-1", "req-1"}}, mgr.denyCalls)
		assert.Empty(t, sigs.addCalls, "deny must not modify the signature ACL")

		if assert.Len(t, ev.logged, 1) {
			logged := ev.logged[0]
			assert.Equal(t, events.ClaManagerAccessRequestDenied, logged.EventType)
			eventData, ok := logged.EventData.(*events.CLAManagerRequestDeniedEventData)
			if assert.True(t, ok) {
				assert.Equal(t, "req-1", eventData.RequestID)
				assert.Equal(t, "Requester", eventData.UserName)
			}
		}

		assert.Len(t, emailSvc.renderCalls, 3, "one email per existing CLA manager plus one to the requester")
		if assert.Len(t, sender.sent, 3) {
			assert.Equal(t, []string{"m1@example.com"}, sender.sent[0].recipients)
			assert.Equal(t, []string{"m2@example.com"}, sender.sent[1].recipients)
			assert.Equal(t, []string{"requester@example.com"}, sender.sent[2].recipients)
			assert.Contains(t, sender.sent[0].subject, "CLA Manager Access Denied Notice for My Project")
			assert.Contains(t, sender.sent[2].subject, "New CLA Manager Access Denied for My Project")
		}
	})

	t.Run("missing request maps to not found", func(t *testing.T) {
		mgr := &fakeManagerService{}
		s := &service{managerService: mgr}

		result, err := s.DenyCLAManagerRequest(context.Background(), authUser, companyModel, "cla-group-1", "req-1")
		assert.Nil(t, result)
		assert.ErrorIs(t, err, errRequestNotFound)
		assert.Empty(t, mgr.denyCalls)
	})
}

func TestClaManagerRequestJSONContract(t *testing.T) {
	body, err := json.Marshal(&models.ClaManagerRequest{})
	assert.Nil(t, err)
	for _, key := range []string{
		"requestID", "companyID", "companyExternalID", "companyName",
		"projectID", "projectExternalID", "projectName",
		"userID", "userExternalID", "userName", "userEmail",
		"status", "created", "updated",
	} {
		assert.Contains(t, string(body), `"`+key+`"`, "zero-value cla manager request must serialize every field")
	}

	listBody, err := json.Marshal(v2ClaManagerRequestList(nil))
	assert.Nil(t, err)
	assert.JSONEq(t, `{"requests":[]}`, string(listBody))
}
