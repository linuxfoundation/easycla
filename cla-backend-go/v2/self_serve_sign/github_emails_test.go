// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package self_serve_sign

import (
	"context"
	"errors"
	"testing"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/my_clas"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeIDPTokens struct {
	token    string
	err      error
	calls    [][3]string
	deadline bool
}

func (f *fakeIDPTokens) IdentityAccessToken(ctx context.Context, lfUsername, provider, userID string) (string, error) {
	_, f.deadline = ctx.Deadline()
	f.calls = append(f.calls, [3]string{lfUsername, provider, userID})
	return f.token, f.err
}

func TestPrepareSignRefreshesGithubEmails(t *testing.T) {
	tests := []struct {
		name       string
		stored     []string
		recordID   string
		noIDs      bool
		tokens     *fakeIDPTokens
		verified   []string
		githubErr  error
		wantLookup bool
		wantGithub bool
		wantEmails []string
	}{
		{name: "stores the verified addresses", stored: []string{"lf@example.org"}, recordID: "26589865", tokens: &fakeIDPTokens{token: "gho_test"},
			verified: []string{" New@Example.org ", "lf@example.org", "new@example.org"}, wantLookup: true, wantGithub: true, wantEmails: []string{"new@example.org"}},
		{name: "keeps a stored address that GitHub does not list", stored: []string{"lf@example.org", "Second@lf.example.org"}, recordID: "26589865", tokens: &fakeIDPTokens{token: "gho_test"},
			verified: []string{"gh@example.org"}, wantLookup: true, wantGithub: true, wantEmails: []string{"gh@example.org"}},
		{name: "request with the LF username only", stored: []string{"lf@example.org"}, recordID: "26589865", noIDs: true, tokens: &fakeIDPTokens{token: "gho_test"},
			verified: []string{"gh@example.org"}, wantLookup: true, wantGithub: true, wantEmails: []string{"gh@example.org"}},
		{name: "same addresses are not written again", stored: []string{"LF@example.org", "new@example.org"}, recordID: "26589865", tokens: &fakeIDPTokens{token: "gho_test"},
			verified: []string{"new@example.org", "lf@example.org"}, wantLookup: true, wantGithub: true},
		{name: "no token without the scope", stored: []string{"lf@example.org"}, recordID: "26589865", tokens: &fakeIDPTokens{}, wantLookup: true},
		{name: "token lookup fails", stored: []string{"lf@example.org"}, recordID: "26589865", tokens: &fakeIDPTokens{err: errors.New("auth0 management api returned status 429")}, wantLookup: true},
		{name: "GitHub refuses", stored: []string{"lf@example.org"}, recordID: "26589865", tokens: &fakeIDPTokens{token: "gho_test"}, githubErr: errors.New("401 Bad credentials"), wantLookup: true, wantGithub: true},
		{name: "no verified address", stored: []string{"lf@example.org"}, recordID: "26589865", tokens: &fakeIDPTokens{token: "gho_test"}, verified: []string{" "}, wantLookup: true, wantGithub: true},
		{name: "record bound to another GitHub account", stored: []string{"lf@example.org"}, recordID: "111", tokens: &fakeIDPTokens{token: "gho_test"}},
		{name: "refresh not configured", stored: []string{"lf@example.org"}, recordID: "26589865"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := &v1Models.User{UserID: "existing-user-id", LfUsername: "lgryglicki", GithubID: tt.recordID, GithubUsername: "octocat", Emails: tt.stored}
			users := &fakeUsers{byLFUsername: map[string]*v1Models.User{"lgryglicki": existing}}
			allowed := &my_clas.Identity{GithubIDs: []int64{26589865}, GithubUsernames: []string{"octocat"}}
			if tt.noIDs {
				allowed = &my_clas.Identity{}
			}
			svc := newTestService(&fakeMyClas{allowed: allowed}, users, enabledCLAGroup(), &fakeStore{})
			if tt.tokens != nil {
				WithGithubEmailRefresh(tt.tokens)(svc)
			}
			githubCalls := 0
			svc.githubVerifiedEmails = func(ctx context.Context, accessToken string) ([]string, error) {
				githubCalls++
				_, bounded := ctx.Deadline()
				assert.True(t, bounded, "a slow GitHub must not hold the signing back")
				assert.Equal(t, "gho_test", accessToken)
				return tt.verified, tt.githubErr
			}

			result, err := svc.PrepareSign(context.Background(), &my_clas.Caller{Username: "lgryglicki"}, "lf@example.org", &models.PrepareSignInput{
				ClaGroupID: stringRef(testCLAGroupID),
				ReturnURL:  uriRef(testReturnURL),
				LfUsername: "lgryglicki",
			})

			require.NoError(t, err)
			assert.Equal(t, "existing-user-id", result.UserID)
			if tt.tokens != nil {
				if tt.wantLookup {
					assert.Equal(t, [][3]string{{"lgryglicki", "github", "26589865"}}, tt.tokens.calls)
					assert.True(t, tt.tokens.deadline, "a slow Auth0 must not hold the signing back")
				} else {
					assert.Empty(t, tt.tokens.calls)
				}
			}
			if tt.wantGithub {
				assert.Equal(t, 1, githubCalls)
			} else {
				assert.Zero(t, githubCalls)
			}
			assert.Nil(t, users.updates, "the e-mail set is never written as a whole")
			assert.Equal(t, tt.wantEmails, users.addedEmails)
		})
	}
}

func TestPrepareSignDoesNotRefreshEmailsForAnotherContributor(t *testing.T) {
	existing := &v1Models.User{UserID: "existing-user-id", GithubID: "26589865", GithubUsername: "octocat", Emails: []string{"other@example.org"}}
	users := &fakeUsers{byGithubID: map[string]*v1Models.User{"26589865": existing}}
	svc := newTestService(&fakeMyClas{allowed: &my_clas.Identity{}}, users, enabledCLAGroup(), &fakeStore{})
	tokens := &fakeIDPTokens{token: "gho_test"}
	WithGithubEmailRefresh(tokens)(svc)
	svc.githubVerifiedEmails = func(context.Context, string) ([]string, error) {
		t.Fatal("the admin's GitHub account must not be read for another contributor")
		return nil, nil
	}

	_, err := svc.PrepareSign(context.Background(), &my_clas.Caller{Username: "admin", Admin: true}, "admin@example.org", &models.PrepareSignInput{
		ClaGroupID:     stringRef(testCLAGroupID),
		ReturnURL:      uriRef(testReturnURL),
		GithubID:       26589865,
		GithubUsername: "octocat",
	})

	require.NoError(t, err)
	assert.Empty(t, tokens.calls)
	assert.Nil(t, users.updates)
	assert.Nil(t, users.addedEmails)
}
