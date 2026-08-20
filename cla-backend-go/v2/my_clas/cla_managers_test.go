// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const someoneEmail = "someone@example.org"

type fakeEvents struct {
	logged []*events.LogEventArgs
}

func (f *fakeEvents) LogEventWithContext(_ context.Context, args *events.LogEventArgs) {
	f.logged = append(f.logged, args)
}

type sentEmail struct {
	subject    string
	body       string
	recipients []string
}

func managersFixture() (*fakeRepo, *fakeSignatures, *fakeCompanies) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone", Username: "Some One"}
	sig := ecla("sig-ecla", "company-1", "2024-01-01T00:00:00Z", true)
	sig.UserEmail = someoneEmail
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {
				sig,
				icla("sig-icla", "user-a", "cla-group-1", "2024-02-01T00:00:00Z", true),
			},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	signaturesService := &fakeSignatures{
		cclas: map[string]*v1Models.Signature{
			"cla-group-1|company-1": {SignatureID: "ccla-1", SignatureACL: []v1Models.User{
				{LfUsername: "manager-one", Username: "Manager One", LfEmail: "manager-one@corp.example.org"},
				{LfUsername: "manager-two", Username: "Manager Two", Emails: []string{"manager-two@corp.example.org"}},
				{Username: "acl-no-lfid"},
				{},
			}},
		},
		approvedUserIDs: map[string]bool{"user-a": true},
	}
	companies := &fakeCompanies{byID: map[string]*v1Models.Company{
		"company-1": {CompanyID: "company-1", CompanyName: "Good Corp"},
	}}
	return repo, signaturesService, companies
}

func TestGetMyClasSignedIdentity(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	github := icla("sig-github", "user-a", "cla-group-1", "2024-01-01T00:00:00Z", true)
	github.UserGithubUsername = "octocat"
	github.UserGithubID = "999"
	githubIDOnly := icla("sig-github-id", "user-a", "cla-group-1", "2024-02-01T00:00:00Z", true)
	githubIDOnly.UserGithubID = "999"
	gitlab := icla("sig-gitlab", "user-a", "cla-group-1", "2024-03-01T00:00:00Z", true)
	gitlab.UserGitlabUsername = "octolab"
	gerrit := icla("sig-gerrit", "user-a", "cla-group-1", "2024-04-01T00:00:00Z", true)
	gerrit.UserEmail = someoneEmail
	sso := icla("sig-sso", "user-a", "cla-group-1", "2024-05-01T00:00:00Z", true)
	sso.UserLFUsername = "someone"
	anonymous := icla("sig-anonymous", "user-a", "cla-group-1", "2024-06-01T00:00:00Z", true)

	repo := &fakeRepo{
		byUserID:     map[string][]*signatures.ItemSignature{"user-a": {github, githubIDOnly, gitlab, gerrit, sso, anonymous}},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err)
	byID := map[string]models.MyCla{}
	for _, row := range result.Clas {
		byID[row.SignatureID] = row
	}

	assert.Equal(t, "github", byID["sig-github"].SignedVia)
	assert.Equal(t, "octocat", byID["sig-github"].SignedAs, "the username wins over the numeric ID")
	assert.Equal(t, "github", byID["sig-github-id"].SignedVia)
	assert.Equal(t, "999", byID["sig-github-id"].SignedAs, "the numeric ID is the fallback account")
	assert.Equal(t, "gitlab", byID["sig-gitlab"].SignedVia)
	assert.Equal(t, "octolab", byID["sig-gitlab"].SignedAs)
	assert.Equal(t, "gerrit", byID["sig-gerrit"].SignedVia)
	assert.Equal(t, someoneEmail, byID["sig-gerrit"].SignedAs)
	assert.Equal(t, "gerrit", byID["sig-sso"].SignedVia, "LF SSO signings surface as gerrit")
	assert.Equal(t, "someone", byID["sig-sso"].SignedAs)
	require.Contains(t, byID, "sig-anonymous", "the identity-less record is still returned")
	assert.Empty(t, byID["sig-anonymous"].SignedVia, "no identity on the record leaves both fields omitted")
	assert.Empty(t, byID["sig-anonymous"].SignedAs)
}

func TestGetMyClasFlaggedAndClaManager(t *testing.T) {
	userA := &v1Models.User{UserID: "user-a", LfUsername: "someone"}
	companies := &fakeCompanies{byID: map[string]*v1Models.Company{
		"company-1": {CompanyID: "company-1", CompanyName: "Good Corp"},
		"company-2": {CompanyID: "company-2", CompanyName: "Sanctioned Corp", IsSanctioned: true},
	}}
	signaturesService := &fakeSignatures{
		cclas: map[string]*v1Models.Signature{
			"cla-group-1|company-1": {SignatureID: "ccla-1", SignatureACL: []v1Models.User{{LfUsername: "SomeOne"}}},
		},
		approvedUserIDs: map[string]bool{"user-a": true},
	}
	repo := &fakeRepo{
		byUserID: map[string][]*signatures.ItemSignature{
			"user-a": {
				ecla("sig-good", "company-1", "2024-01-01T00:00:00Z", true),
				ecla("sig-sanctioned", "company-2", "2024-02-01T00:00:00Z", true),
				icla("sig-icla", "user-a", "cla-group-1", "2024-03-01T00:00:00Z", true),
			},
		},
		byLFUsername: map[string][]*v1Models.User{"someone": {userA}},
	}
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err)
	byID := map[string]models.MyCla{}
	for _, row := range result.Clas {
		byID[row.SignatureID] = row
	}

	good := byID["sig-good"]
	assert.False(t, good.Flagged)
	assert.Empty(t, good.FlaggedAt)
	assert.True(t, good.ClaManager, "the ACL match is case-insensitive")
	assert.True(t, good.Valid)

	sanctioned := byID["sig-sanctioned"]
	assert.True(t, sanctioned.Flagged, "a sanctioned employer flags the ECLA")
	assert.NotEmpty(t, sanctioned.FlaggedAt)
	assert.Equal(t, models.MyClaStatusRevoked, sanctioned.Status, "the Revoked state is system-set from sanctions")
	assert.False(t, sanctioned.Valid)
	assert.False(t, sanctioned.ClaManager, "a sanctioned employer carries no CLA manager action")

	regular := byID["sig-icla"]
	assert.False(t, regular.Flagged, "ICLAs are never flagged")
	assert.False(t, regular.ClaManager)
}

func TestGetMyClasNotAClaManager(t *testing.T) {
	repo, signaturesService, companies := managersFixture()
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})

	result, err := svc.GetMyClas(context.Background(), &Caller{Username: "someone"}, &Identity{})
	require.NoError(t, err)
	for _, row := range result.Clas {
		assert.False(t, row.ClaManager)
	}
}

func TestGetMyClaManagers(t *testing.T) {
	repo, signaturesService, companies := managersFixture()
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{names: map[string]string{"cla-group-1": "My CLA Group"}})
	caller := &Caller{Username: "someone"}

	result, err := svc.GetMyClaManagers(context.Background(), caller, &Identity{}, "sig-ecla")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "sig-ecla", result.SignatureID)
	assert.Equal(t, "cla-group-1", result.ClaGroupID)
	assert.Equal(t, "My CLA Group", result.ClaGroupName)
	assert.Equal(t, "company-1", result.CompanyID)
	assert.Equal(t, "Good Corp", result.CompanyName)
	assert.False(t, result.ClaManager)
	assert.Equal(t, int64(3), result.ResultCount)
	require.Len(t, result.Managers, 3)
	assert.Equal(t, models.MyClaManager{LfUsername: "manager-one", Name: "Manager One", Email: "manager-one@corp.example.org"}, result.Managers[0])
	assert.Equal(t, models.MyClaManager{LfUsername: "manager-two", Name: "Manager Two", Email: "manager-two@corp.example.org"}, result.Managers[1], "the additional-emails list is the email fallback")
	assert.Equal(t, models.MyClaManager{LfUsername: "acl-no-lfid", Name: "acl-no-lfid"}, result.Managers[2], "the plain username is the LF username fallback")

	result, err = svc.GetMyClaManagers(context.Background(), caller, &Identity{}, "sig-icla")
	require.NoError(t, err)
	assert.Nil(t, result, "ICLAs have no CLA managers")

	result, err = svc.GetMyClaManagers(context.Background(), caller, &Identity{}, "sig-of-somebody-else")
	require.NoError(t, err)
	assert.Nil(t, result, "signatures not owned by the resolved identity are not found")
}

func TestGetMyClaManagersCallerIsManager(t *testing.T) {
	repo, signaturesService, companies := managersFixture()
	ccla := signaturesService.cclas["cla-group-1|company-1"]
	ccla.SignatureACL = append(ccla.SignatureACL, v1Models.User{LfUsername: "SomeOne"})
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})

	result, err := svc.GetMyClaManagers(context.Background(), &Caller{Username: "someone"}, &Identity{}, "sig-ecla")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.ClaManager, "the caller shows up as a CLA manager, case-insensitively")
}

func TestGetMyClaManagersNoCcla(t *testing.T) {
	repo, _, companies := managersFixture()
	svc := newTestService(repo, &fakePlatform{}, &fakeSignatures{}, companies, &fakeClaGroups{})

	result, err := svc.GetMyClaManagers(context.Background(), &Caller{Username: "someone"}, &Identity{}, "sig-ecla")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Empty(t, result.Managers, "no current CCLA yields an empty manager list")
	assert.Equal(t, int64(0), result.ResultCount)
	assert.Equal(t, "Good Corp", result.CompanyName)
}

func TestGetMyClaManagersOwnershipEnforced(t *testing.T) {
	repo, signaturesService, companies := managersFixture()
	victim := &v1Models.User{UserID: "user-v", LfUsername: "victim"}
	repo.byLFUsername["victim"] = []*v1Models.User{victim}
	repo.byUserID["user-v"] = []*signatures.ItemSignature{ecla("sig-victim", "company-1", "2024-01-01T00:00:00Z", true)}
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{})

	result, err := svc.GetMyClaManagers(context.Background(), &Caller{Username: "someone"}, &Identity{LfUsername: "victim"}, "sig-victim")
	require.NoError(t, err)
	assert.Nil(t, result, "a non-admin cannot resolve somebody else's ECLA")

	result, err = svc.GetMyClaManagers(context.Background(), &Caller{Username: "staff-admin", Admin: true}, &Identity{LfUsername: "victim"}, "sig-victim")
	require.NoError(t, err)
	require.NotNil(t, result, "an admin can")
}

func requestInput(requestType string, recipients []string, message string) *models.MyClaManagerRequest {
	return &models.MyClaManagerRequest{RequestType: &requestType, Recipients: recipients, Message: message}
}

func newRequestTestService(repo *fakeRepo, signaturesService SignaturesService, companies CompanyRepository) (*service, *fakeEvents, *[]sentEmail) {
	svc := newTestService(repo, &fakePlatform{}, signaturesService, companies, &fakeClaGroups{names: map[string]string{"cla-group-1": "My CLA Group"}})
	eventsService := &fakeEvents{}
	svc.eventsService = eventsService
	sent := &[]sentEmail{}
	svc.sendEmail = func(subject, body string, recipients []string) error {
		*sent = append(*sent, sentEmail{subject: subject, body: body, recipients: recipients})
		return nil
	}
	return svc, eventsService, sent
}

func TestCreateMyClaManagerRequest(t *testing.T) {
	repo, signaturesService, companies := managersFixture()
	svc, eventsService, sent := newRequestTestService(repo, signaturesService, companies)
	caller := &Caller{Username: "someone"}

	result, err := svc.CreateMyClaManagerRequest(context.Background(), caller, &Identity{}, "sig-ecla",
		requestInput("removal", []string{"Manager-One", "manager-two"}, "please remove me"))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.RequestID)
	assert.Equal(t, "sig-ecla", result.SignatureID)
	assert.Equal(t, "removal", result.RequestType)
	assert.Equal(t, "sent", result.Status)
	assert.Equal(t, []string{"manager-one", "manager-two"}, result.Recipients, "recipients echo the canonical manager usernames")

	require.Len(t, *sent, 1)
	email := (*sent)[0]
	assert.Equal(t, []string{"manager-one@corp.example.org", "manager-two@corp.example.org"}, email.recipients)
	assert.Contains(t, email.subject, "removal from the corporate CLA coverage")
	assert.Contains(t, email.body, "Some One")
	assert.Contains(t, email.body, someoneEmail)
	assert.Contains(t, email.body, "Good Corp")
	assert.Contains(t, email.body, "please remove me")

	require.Len(t, eventsService.logged, 1)
	logged := eventsService.logged[0]
	assert.Equal(t, events.ContactCLAManagerRequestCreated, logged.EventType)
	assert.Equal(t, "user-a", logged.UserID)
	assert.Equal(t, "company-1", logged.CompanyID)
	eventData, ok := logged.EventData.(*events.ContactCLAManagerRequestCreatedEventData)
	require.True(t, ok)
	assert.Equal(t, result.RequestID, eventData.RequestID)
	assert.Equal(t, "removal", eventData.RequestType)
	assert.Equal(t, "please remove me", eventData.Message, "the audit event is the receipt, so it carries the message")
	assert.Equal(t, []string{"manager-one", "manager-two"}, eventData.Recipients)
}

func TestCreateMyClaManagerRequestApprovalWording(t *testing.T) {
	repo, signaturesService, companies := managersFixture()
	svc, _, sent := newRequestTestService(repo, signaturesService, companies)

	result, err := svc.CreateMyClaManagerRequest(context.Background(), &Caller{Username: "someone"}, &Identity{}, "sig-ecla",
		requestInput("approval", []string{"manager-one"}, ""))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "approval", result.RequestType)
	require.Len(t, *sent, 1)
	assert.Contains(t, (*sent)[0].body, "approval under the corporate CLA")
}

func TestCreateMyClaManagerRequestRecipientValidation(t *testing.T) {
	repo, signaturesService, companies := managersFixture()
	svc, eventsService, sent := newRequestTestService(repo, signaturesService, companies)
	caller := &Caller{Username: "someone"}

	_, err := svc.CreateMyClaManagerRequest(context.Background(), caller, &Identity{}, "sig-ecla",
		requestInput("removal", nil, ""))
	assert.ErrorIs(t, err, ErrInvalidRecipients, "recipients are required while managers resolve")

	_, err = svc.CreateMyClaManagerRequest(context.Background(), caller, &Identity{}, "sig-ecla",
		requestInput("removal", []string{"manager-one", "outsider"}, ""))
	assert.ErrorIs(t, err, ErrInvalidRecipients, "a recipient outside the resolved managers is rejected")

	assert.Empty(t, *sent)
	assert.Empty(t, eventsService.logged)
}

func TestCreateMyClaManagerRequestRecipientDedupe(t *testing.T) {
	repo, signaturesService, companies := managersFixture()
	svc, _, sent := newRequestTestService(repo, signaturesService, companies)
	caller := &Caller{Username: "someone"}

	result, err := svc.CreateMyClaManagerRequest(context.Background(), caller, &Identity{}, "sig-ecla",
		requestInput("removal", []string{"Manager-One", "manager-one"}, ""))
	require.NoError(t, err)
	assert.Equal(t, []string{"manager-one"}, result.Recipients, "case-variant duplicates collapse to one recipient")

	require.Len(t, *sent, 1)
	assert.Equal(t, []string{"manager-one@corp.example.org"}, (*sent)[0].recipients)
}

func TestCreateMyClaManagerRequestZeroManagers(t *testing.T) {
	repo, _, companies := managersFixture()
	svc, eventsService, sent := newRequestTestService(repo, &fakeSignatures{}, companies)
	caller := &Caller{Username: "someone"}

	_, err := svc.CreateMyClaManagerRequest(context.Background(), caller, &Identity{}, "sig-ecla",
		requestInput("removal", []string{"manager-one"}, ""))
	assert.ErrorIs(t, err, ErrInvalidRecipients, "recipients must be empty when no manager resolves")

	result, err := svc.CreateMyClaManagerRequest(context.Background(), caller, &Identity{}, "sig-ecla",
		requestInput("removal", nil, "nobody to write to"))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "recorded", result.Status, "the request is recorded without sending email")
	assert.Empty(t, result.Recipients)
	assert.Empty(t, *sent)
	require.Len(t, eventsService.logged, 1, "the audit event is still logged")
}

func TestCreateMyClaManagerRequestNotFound(t *testing.T) {
	repo, signaturesService, companies := managersFixture()
	svc, _, _ := newRequestTestService(repo, signaturesService, companies)
	caller := &Caller{Username: "someone"}

	result, err := svc.CreateMyClaManagerRequest(context.Background(), caller, &Identity{}, "sig-icla",
		requestInput("removal", []string{"manager-one"}, ""))
	require.NoError(t, err)
	assert.Nil(t, result, "an ICLA has no CLA managers to contact")

	result, err = svc.CreateMyClaManagerRequest(context.Background(), caller, &Identity{}, "sig-unknown",
		requestInput("removal", []string{"manager-one"}, ""))
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestCreateMyClaManagerRequestSendFailure(t *testing.T) {
	repo, signaturesService, companies := managersFixture()
	svc, eventsService, _ := newRequestTestService(repo, signaturesService, companies)
	svc.sendEmail = func(_, _ string, _ []string) error { return errors.New("SNS unavailable") }

	_, err := svc.CreateMyClaManagerRequest(context.Background(), &Caller{Username: "someone"}, &Identity{}, "sig-ecla",
		requestInput("removal", []string{"manager-one"}, ""))
	assert.Error(t, err, "a send failure is surfaced to the caller")
	assert.Empty(t, eventsService.logged, "no audit event is logged when the email fails")
}

func TestCreateMyClaManagerRequestManagerWithoutEmail(t *testing.T) {
	repo, signaturesService, companies := managersFixture()
	svc, _, sent := newRequestTestService(repo, signaturesService, companies)

	result, err := svc.CreateMyClaManagerRequest(context.Background(), &Caller{Username: "someone"}, &Identity{}, "sig-ecla",
		requestInput("removal", []string{"acl-no-lfid"}, ""))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "recorded", result.Status, "a selected manager with no email leaves nothing to send")
	assert.Equal(t, []string{"acl-no-lfid"}, result.Recipients)
	assert.Empty(t, *sent)
}

func TestCreateMyClaManagerRequestMixedRecipientEmails(t *testing.T) {
	repo, signaturesService, companies := managersFixture()
	svc, eventsService, sent := newRequestTestService(repo, signaturesService, companies)

	result, err := svc.CreateMyClaManagerRequest(context.Background(), &Caller{Username: "someone"}, &Identity{}, "sig-ecla",
		requestInput("removal", []string{"manager-one", "acl-no-lfid"}, ""))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "sent", result.Status, "one reachable manager is enough for sent")
	assert.Equal(t, []string{"manager-one", "acl-no-lfid"}, result.Recipients, "recipients are the whole selection, reachable or not")

	require.Len(t, *sent, 1)
	assert.Equal(t, []string{"manager-one@corp.example.org"}, (*sent)[0].recipients, "only the reachable manager is emailed")

	require.Len(t, eventsService.logged, 1)
	eventData, ok := eventsService.logged[0].EventData.(*events.ContactCLAManagerRequestCreatedEventData)
	require.True(t, ok)
	assert.Equal(t, []string{"manager-one", "acl-no-lfid"}, eventData.Recipients, "the receipt records the whole selection")
}

func decodeJSON(t *testing.T, payload interface{}) map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	decoded := map[string]interface{}{}
	require.NoError(t, json.Unmarshal(raw, &decoded))
	return decoded
}

// TestMyClasJSONContract pins the false and empty values the console keys on - they must appear
// in the payload rather than being dropped by omitempty
func TestMyClasJSONContract(t *testing.T) {
	repo, _, companies := managersFixture()
	svc, _, _ := newRequestTestService(repo, &fakeSignatures{}, companies)
	caller := &Caller{Username: "someone"}

	list, err := svc.GetMyClas(context.Background(), caller, &Identity{})
	require.NoError(t, err)
	byID := map[string]models.MyCla{}
	for _, row := range list.Clas {
		byID[row.SignatureID] = row
	}
	for _, signatureID := range []string{"sig-ecla", "sig-icla"} {
		row := decodeJSON(t, byID[signatureID])
		for _, field := range []string{"flagged", "claManager"} {
			require.Contains(t, row, field, "%s must always carry %s", signatureID, field)
			assert.Equal(t, false, row[field])
		}
	}

	managers, err := svc.GetMyClaManagers(context.Background(), caller, &Identity{}, "sig-ecla")
	require.NoError(t, err)
	decoded := decodeJSON(t, managers)
	require.Contains(t, decoded, "managers")
	assert.NotNil(t, decoded["managers"], "an empty manager list serializes as [], never null")
	assert.Empty(t, decoded["managers"])

	receipt, err := svc.CreateMyClaManagerRequest(context.Background(), caller, &Identity{}, "sig-ecla",
		requestInput("removal", nil, ""))
	require.NoError(t, err)
	decoded = decodeJSON(t, receipt)
	require.Contains(t, decoded, "recipients")
	assert.NotNil(t, decoded["recipients"], "a recorded receipt serializes recipients as []")
	assert.Empty(t, decoded["recipients"])
}

func TestSignedIdentityFallbacks(t *testing.T) {
	gitlabIDOnly := &signatures.ItemSignature{UserGitlabID: "42"}
	via, as := signedIdentity(gitlabIDOnly)
	assert.Equal(t, "gitlab", via)
	assert.Equal(t, "42", as)

	assert.False(t, isClaManager(nil, "someone"))
	assert.False(t, isClaManager(&v1Models.Signature{SignatureACL: []v1Models.User{{LfUsername: "someone"}}}, ""))
	assert.True(t, isClaManager(&v1Models.Signature{SignatureACL: []v1Models.User{{Username: "SomeOne"}}}, "someone"),
		"the plain username matches too")
}
