// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/cla_admins"
	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

const (
	accountA = "0014100000Te0G7AAJ"
	accountB = "0014100000Te0G8AAJ"
)

type rosterCall struct {
	sfid string
	ids  []string
}

type fakeRosterPublisher struct {
	calls []rosterCall
	res   *cla_admins.Result
	err   error
}

func (f *fakeRosterPublisher) Publish(_ context.Context, sfid string, ids ...string) (*cla_admins.Result, error) {
	f.calls = append(f.calls, rosterCall{sfid: sfid, ids: ids})
	return f.res, f.err
}

type alertCall struct {
	subject    string
	body       string
	recipients []string
}

type fakeAlerts struct {
	calls []alertCall
	err   error
}

func (f *fakeAlerts) SendEmail(subject, body string, recipients []string) error {
	f.calls = append(f.calls, alertCall{subject: subject, body: body, recipients: recipients})
	return f.err
}

func cclaImageFor(company string, signed bool, acl ...string) map[string]events.DynamoDBAttributeValue {
	members := make([]events.DynamoDBAttributeValue, 0, len(acl))
	for _, m := range acl {
		members = append(members, events.NewStringAttribute(m))
	}
	return map[string]events.DynamoDBAttributeValue{
		"signature_id":             events.NewStringAttribute("sig-ccla"),
		"signature_type":           events.NewStringAttribute("ccla"),
		"signature_reference_type": events.NewStringAttribute("company"),
		"signature_reference_id":   events.NewStringAttribute(company),
		"signature_signed":         events.NewBooleanAttribute(signed),
		"signature_acl":            events.NewListAttribute(members),
	}
}

func cclaImage(signed bool, acl ...string) map[string]events.DynamoDBAttributeValue {
	return cclaImageFor("company-1", signed, acl...)
}

func iclaImage() map[string]events.DynamoDBAttributeValue {
	image := cclaImage(true, "amy")
	image["signature_type"] = events.NewStringAttribute("cla")
	image["signature_reference_type"] = events.NewStringAttribute("user")
	return image
}

func withNote(image map[string]events.DynamoDBAttributeValue) map[string]events.DynamoDBAttributeValue {
	image["note"] = events.NewStringAttribute("metadata only")
	return image
}

func modify(oldImage, newImage map[string]events.DynamoDBAttributeValue) events.DynamoDBEventRecord {
	return events.DynamoDBEventRecord{EventName: Modify, Change: events.DynamoDBStreamRecord{OldImage: oldImage, NewImage: newImage}}
}

func insert(newImage map[string]events.DynamoDBAttributeValue) events.DynamoDBEventRecord {
	return events.DynamoDBEventRecord{EventName: Insert, Change: events.DynamoDBStreamRecord{NewImage: newImage}}
}

func remove(oldImage map[string]events.DynamoDBAttributeValue) events.DynamoDBEventRecord {
	return events.DynamoDBEventRecord{EventName: Remove, Change: events.DynamoDBStreamRecord{OldImage: oldImage}}
}

func TestCLAAdminsEvent(t *testing.T) {
	companyA := &models.Company{CompanyID: "company-1", CompanyExternalID: accountA}
	companyB := &models.Company{CompanyID: "company-2", CompanyExternalID: accountB}
	published := &cla_admins.Result{Roster: &cla_admins.Roster{SFID: accountA, Admins: []member_service.CLAAdmin{{Username: "amy"}}}}
	ok := func() *fakeRosterPublisher { return &fakeRosterPublisher{res: published} }
	ops := []string{"ops@example.com", "maintainers@example.com"}
	tests := []struct {
		name       string
		event      events.DynamoDBEventRecord
		companies  map[string]*models.Company
		companyErr error
		publish    *fakeRosterPublisher
		alerts     *fakeAlerts
		recipients []string
		wantCalls  []rosterCall
		wantErr    string
		wantAlert  bool
	}{
		{name: "manager added", event: modify(cclaImage(true, "amy"), cclaImage(true, "amy", "bob")), companies: map[string]*models.Company{"company-1": companyA}, publish: ok(),
			wantCalls: []rosterCall{{sfid: accountA, ids: []string{"sig-ccla"}}}},
		{name: "signed flag cleared", event: modify(cclaImage(true, "amy"), cclaImage(false, "amy")), companies: map[string]*models.Company{"company-1": companyA}, publish: ok(),
			wantCalls: []rosterCall{{sfid: accountA, ids: []string{"sig-ccla"}}}},
		{name: "signed flag set", event: modify(cclaImage(false, "amy"), cclaImage(true, "amy")), companies: map[string]*models.Company{"company-1": companyA}, publish: ok(),
			wantCalls: []rosterCall{{sfid: accountA, ids: []string{"sig-ccla"}}}},
		{name: "same managers in another order", event: modify(cclaImage(true, "amy", "bob"), cclaImage(true, "bob", "amy")), publish: ok()},
		{name: "metadata-only change", event: modify(cclaImage(true, "amy"), withNote(cclaImage(true, "amy"))), publish: ok()},
		{name: "signed ccla inserted", event: insert(cclaImage(true, "amy")), companies: map[string]*models.Company{"company-1": companyA}, publish: ok(),
			wantCalls: []rosterCall{{sfid: accountA, ids: []string{"sig-ccla"}}}},
		{name: "unsigned ccla inserted", event: insert(cclaImage(false, "amy")), publish: ok()},
		{name: "signed ccla removed", event: remove(cclaImage(true, "amy")), companies: map[string]*models.Company{"company-1": companyA}, publish: ok(),
			wantCalls: []rosterCall{{sfid: accountA, ids: []string{"sig-ccla"}}}},
		{name: "unsigned ccla removed", event: remove(cclaImage(false, "amy")), publish: ok()},
		{name: "row moved to another company", event: modify(cclaImage(true, "amy"), cclaImageFor("company-2", true, "amy")), companies: map[string]*models.Company{"company-1": companyA, "company-2": companyB}, publish: ok(),
			wantCalls: []rosterCall{{sfid: accountA, ids: []string{"sig-ccla"}}, {sfid: accountB, ids: []string{"sig-ccla"}}}},
		{name: "row stops being a corporate cla", event: modify(cclaImage(true, "amy"), iclaImage()), companies: map[string]*models.Company{"company-1": companyA}, publish: ok(),
			wantCalls: []rosterCall{{sfid: accountA, ids: []string{"sig-ccla"}}}},
		{name: "company without account id", event: modify(cclaImage(true), cclaImage(true, "amy")), companies: map[string]*models.Company{"company-1": {CompanyID: "company-1"}}, publish: ok()},
		{name: "legacy org-service id", event: modify(cclaImage(true), cclaImage(true, "amy")), companies: map[string]*models.Company{"company-1": {CompanyID: "company-1", CompanyExternalID: "lfSclQhURLCCrv7EZj"}},
			publish: &fakeRosterPublisher{err: member_service.ErrInvalidSFID}, alerts: &fakeAlerts{}, recipients: ops,
			wantCalls: []rosterCall{{sfid: "lfSclQhURLCCrv7EZj", ids: []string{"sig-ccla"}}}},
		{name: "not a b2b_org yet", event: modify(cclaImage(true), cclaImage(true, "amy")), companies: map[string]*models.Company{"company-1": companyA},
			publish: &fakeRosterPublisher{res: &cla_admins.Result{Roster: published.Roster, Skipped: true}}, alerts: &fakeAlerts{}, recipients: ops,
			wantCalls: []rosterCall{{sfid: accountA, ids: []string{"sig-ccla"}}}},
		{name: "member-service failure is returned and reported", event: modify(cclaImage(true), cclaImage(true, "amy")), companies: map[string]*models.Company{"company-1": companyA},
			publish: &fakeRosterPublisher{err: errors.New("boom")}, alerts: &fakeAlerts{}, recipients: ops,
			wantCalls: []rosterCall{{sfid: accountA, ids: []string{"sig-ccla"}}}, wantErr: "boom", wantAlert: true},
		{name: "member-service failure without recipients is only logged", event: modify(cclaImage(true), cclaImage(true, "amy")), companies: map[string]*models.Company{"company-1": companyA},
			publish: &fakeRosterPublisher{err: errors.New("boom")}, alerts: &fakeAlerts{},
			wantCalls: []rosterCall{{sfid: accountA, ids: []string{"sig-ccla"}}}, wantErr: "boom"},
		{name: "a failed alert does not hide the publish failure", event: modify(cclaImage(true), cclaImage(true, "amy")), companies: map[string]*models.Company{"company-1": companyA},
			publish: &fakeRosterPublisher{err: errors.New("boom")}, alerts: &fakeAlerts{err: errors.New("sns down")}, recipients: ops,
			wantCalls: []rosterCall{{sfid: accountA, ids: []string{"sig-ccla"}}}, wantErr: "boom", wantAlert: true},
		{name: "company lookup failure is returned and reported", event: modify(cclaImage(true), cclaImage(true, "amy")), companyErr: errors.New("dynamo down"),
			publish: ok(), alerts: &fakeAlerts{}, recipients: ops, wantErr: "dynamo down", wantAlert: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			companyRepo := mock_company.NewMockIRepository(ctrl)
			companyRepo.EXPECT().GetCompany(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, id string) (*models.Company, error) {
				if tc.companyErr != nil {
					return nil, tc.companyErr
				}
				c, found := tc.companies[id]
				require.Truef(t, found, "unexpected company lookup: %s", id)
				return c, nil
			}).AnyTimes()
			s := &service{companyRepo: companyRepo, claAdmins: tc.publish, claAdminsRecipients: tc.recipients}
			if tc.alerts != nil {
				s.claAdminsAlerts = tc.alerts
			}
			err := s.CLAAdminsEvent(tc.event)
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.wantCalls, tc.publish.calls)
			if tc.alerts == nil {
				return
			}
			if !tc.wantAlert {
				assert.Empty(t, tc.alerts.calls)
				return
			}
			require.Len(t, tc.alerts.calls, 1)
			assert.Equal(t, tc.recipients, tc.alerts.calls[0].recipients)
			assert.Contains(t, tc.alerts.calls[0].subject, "CLA-manager roster not published")
			assert.Contains(t, tc.alerts.calls[0].body, "sig-ccla")
			assert.Contains(t, tc.alerts.calls[0].body, tc.wantErr)
		})
	}
}

func TestCLAAdminsEventIgnoresOtherRows(t *testing.T) {
	pub := &fakeRosterPublisher{}
	s := &service{claAdmins: pub}
	require.NoError(t, s.CLAAdminsEvent(modify(cclaImage(false), iclaImage())))
	require.NoError(t, s.CLAAdminsEvent(insert(iclaImage())))
	assert.Empty(t, pub.calls)

	off := &service{}
	require.NoError(t, off.CLAAdminsEvent(modify(cclaImage(true), cclaImage(true, "amy"))), "without the option the handler is a no-op")
}

func TestWithCLAAdminsRegistersTheSignatureEvents(t *testing.T) {
	s := &service{functions: map[string][]EventHandlerFunc{}, signaturesTable: "cla-dev-signatures"}
	alerts := &fakeAlerts{}
	WithCLAAdmins(&fakeRosterPublisher{}, alerts, []string{"ops@example.com"})(s)
	for _, eventName := range []string{Insert, Modify, Remove} {
		assert.Len(t, s.functions["cla-dev-signatures:"+eventName], 1, eventName)
	}
	assert.Same(t, alerts, s.claAdminsAlerts)
	assert.Equal(t, []string{"ops@example.com"}, s.claAdminsRecipients)
}

func TestAlertRecipients(t *testing.T) {
	assert.Equal(t, []string{"a@example.com", "b@example.com"}, AlertRecipients(" a@example.com, b@example.com ,,"))
	assert.Nil(t, AlertRecipients(""))
}
