// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"errors"
	"testing"

	"github.com/go-openapi/strfmt"
	"github.com/golang/mock/gomock"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	v2CompanyOps "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/company"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	v2UserService "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service"
	v2UserServiceModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCallerUsername = "jane"
	testCallerEmail    = "jane@example.invalid"
)

// the user ID in the path must be the signed-in user's own EasyCLA user, proven by the token identity (lfx-self-serve-ops#343)
func TestCallerOwnsUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mock_users.NewMockUserRepository(ctrl)
	jane := &v1Models.User{UserID: "u-jane", LfUsername: "jane", LfEmail: strfmt.Email("jane@example.invalid"), Emails: []string{"jane.alt@example.invalid"}}
	githubOnly := &v1Models.User{UserID: "u-gh", Emails: []string{"gh@example.invalid"}}
	repo.EXPECT().GetUser("u-jane").Return(jane, nil).AnyTimes()
	repo.EXPECT().GetUser("u-gh").Return(githubOnly, nil).AnyTimes()
	repo.EXPECT().GetUser("u-ghost").Return(nil, nil)
	repo.EXPECT().GetUser("u-down").Return(nil, errors.New("dynamodb unavailable"))
	svc := &service{userRepo: repo}
	str := func(s string) *string { return &s }

	assert.NoError(t, svc.callerOwnsUser("u-jane", str("Jane"), str("other@example.invalid")), "LF username proves ownership")
	assert.NoError(t, svc.callerOwnsUser("u-jane", str("mallory"), str("JANE@example.invalid")), "token e-mail matching lf_email proves ownership")
	assert.NoError(t, svc.callerOwnsUser("u-jane", nil, str("jane.alt@example.invalid")), "token e-mail matching an alternate e-mail proves ownership")
	assert.NoError(t, svc.callerOwnsUser("u-gh", nil, str("gh@example.invalid")), "a GitHub-only row is owned through its public e-mail")
	err := svc.callerOwnsUser("u-jane", str("mallory"), str("mallory@example.invalid"))
	assert.True(t, errors.Is(err, ErrCompanyUserMismatch), "another user's ID is refused")
	err = svc.callerOwnsUser("u-gh", str("mallory"), nil)
	assert.True(t, errors.Is(err, ErrCompanyUserMismatch), "a GitHub-only row is not owned by an unrelated token")
	err = svc.callerOwnsUser("u-jane", nil, nil)
	assert.True(t, errors.Is(err, ErrCompanyUserMismatch), "a token without username or e-mail owns nothing")
	err = svc.callerOwnsUser("u-ghost", str("jane"), str("jane@example.invalid"))
	assert.True(t, errors.Is(err, ErrCompanyUserMismatch), "an unknown user ID is refused")
	err = svc.callerOwnsUser("u-down", str("jane"), str("jane@example.invalid"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrCompanyUserLookup), "a lookup failure is reported as such, not as a mismatch: %v", err)
	assert.False(t, errors.Is(err, ErrCompanyUserMismatch))
}

// without a gateway-verified e-mail there is nothing to grant company-admin to: the request is refused before any lookup or write
func TestCreateCompanyRefusesWithoutAVerifiedEmail(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mock_users.NewMockUserRepository(ctrl) // no expectations: any lookup fails the test
	svc := &service{userRepo: repo}
	name, site, username := testCompanyName, testCompanySite, testCallerUsername
	email := strfmt.Email("jane@example.invalid")
	_, err := svc.CreateCompany(context.Background(), &v2CompanyOps.CreateCompanyParams{
		UserID:    "u-jane",
		XUSERNAME: &username,
		Input:     &models.CompanyInput{CompanyName: &name, CompanyWebsite: &site, UserEmail: &email},
	})
	assert.True(t, errors.Is(err, ErrCompanyNoGrantTarget), "%v", err)
}

// the verified e-mail must resolve to an LF login before anything is created: no user -> 403, lookup outage -> 500
func TestCreateCompanyRequiresAnLFUserForTheGrant(t *testing.T) {
	previous := lfUserByEmail
	t.Cleanup(func() { lfUserByEmail = previous })
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mock_users.NewMockUserRepository(ctrl)
	repo.EXPECT().GetUser("u-jane").Return(&v1Models.User{UserID: "u-jane", LfUsername: "jane", LfEmail: strfmt.Email("jane@example.invalid")}, nil).AnyTimes()
	svc := &service{userRepo: repo}
	call := func() error {
		name, site, username, email := testCompanyName, testCompanySite, testCallerUsername, testCallerEmail
		bodyEmail := strfmt.Email(email)
		_, err := svc.CreateCompany(context.Background(), &v2CompanyOps.CreateCompanyParams{
			UserID: "u-jane", XUSERNAME: &username, XEMAIL: &email,
			Input: &models.CompanyInput{CompanyName: &name, CompanyWebsite: &site, UserEmail: &bodyEmail},
		})
		return err
	}
	lfUserByEmail = func(string) (*v2UserServiceModels.User, error) { return nil, v2UserService.ErrUserNotFound }
	assert.True(t, errors.Is(call(), ErrCompanyNoGrantTarget), "no LF login behind the verified e-mail")
	lfUserByEmail = func(string) (*v2UserServiceModels.User, error) { return &v2UserServiceModels.User{}, nil }
	assert.True(t, errors.Is(call(), ErrCompanyNoGrantTarget), "an LF user without a username cannot be granted")
	lfUserByEmail = func(string) (*v2UserServiceModels.User, error) { return nil, errors.New("user-service down") }
	assert.True(t, errors.Is(call(), ErrCompanyUserLookup), "a lookup outage is reported, not swallowed")
}

// the grant lookup is keyed by the gateway-verified e-mail, never by the body e-mail
func TestCreateCompanyLooksUpTheVerifiedEmailNotTheBodyOne(t *testing.T) {
	previous := lfUserByEmail
	t.Cleanup(func() { lfUserByEmail = previous })
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	repo := mock_users.NewMockUserRepository(ctrl)
	repo.EXPECT().GetUser("u-jane").Return(&v1Models.User{UserID: "u-jane", LfUsername: testCallerUsername}, nil)
	var lookedUp []string
	lfUserByEmail = func(email string) (*v2UserServiceModels.User, error) {
		lookedUp = append(lookedUp, email)
		return nil, errors.New("stop here")
	}
	name, site, username, verified := testCompanyName, testCompanySite, testCallerUsername, testCallerEmail
	bodyEmail := strfmt.Email("mallory@example.invalid")
	_, err := (&service{userRepo: repo}).CreateCompany(context.Background(), &v2CompanyOps.CreateCompanyParams{
		UserID: "u-jane", XUSERNAME: &username, XEMAIL: &verified,
		Input: &models.CompanyInput{CompanyName: &name, CompanyWebsite: &site, UserEmail: &bodyEmail},
	})
	require.Error(t, err)
	assert.Equal(t, []string{testCallerEmail}, lookedUp, "the body e-mail cannot steer the grant lookup")
}

// the LF user verified before the Salesforce write is the one that gets company-admin: no second lookup that could fail
func TestGrantUserForUsesTheVerifiedUser(t *testing.T) {
	previous := lfUserByEmail
	t.Cleanup(func() { lfUserByEmail = previous })
	verified := &v2UserServiceModels.User{Username: testCallerUsername}
	lfUserByEmail = func(string) (*v2UserServiceModels.User, error) {
		t.Fatal("the verified user must not be looked up again")
		return nil, nil
	}
	assert.Same(t, verified, grantUserFor(logrus.Fields{}, verified, testCallerEmail))
	looked := &v2UserServiceModels.User{Username: "owner"}
	lfUserByEmail = func(string) (*v2UserServiceModels.User, error) { return looked, nil }
	assert.Same(t, looked, grantUserFor(logrus.Fields{}, nil, "owner@example.invalid"), "the SF-model path still looks the owner up")
	assert.Nil(t, grantUserFor(logrus.Fields{}, nil, ""))
}
