// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package member_service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const liveTestSFID = "0014100000Te0G7AAJ"

var liveTestOrg = map[string]string{"uid": liveTestSFID, "name": "Example Org"}

func TestLiveAccounts(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		response interface{}
		live     bool
	}{
		{"200 is live", http.StatusOK, liveTestOrg, true},
		{"404 is not live", http.StatusNotFound, nil, false},
		{"403 is not live", http.StatusForbidden, nil, false},
		{"422 is not live", http.StatusUnprocessableEntity, nil, false},
		{"500 is not live", http.StatusInternalServerError, nil, false},
		{"200 without an Account is not live", http.StatusOK, map[string]string{"name": "Example Org"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeMemberService{status: tt.status, response: tt.response}
			accounts := NewLiveAccounts(newTestClient(t, fake))
			assert.Equal(t, tt.live, accounts.IsLive(context.Background(), liveTestSFID))
			assert.Equal(t, tt.live, accounts.IsLive(context.Background(), " 0014100000Te0G7 "), "the 15-character form names the same Account")
			assert.Equal(t, 1, fake.getCalls, "the answer is kept")
			assert.Equal(t, []string{"/b2b_orgs/" + liveTestSFID}, fake.getPaths)
		})
	}
}

func TestLiveAccountsAskAgainWhenTheAnswerIsOld(t *testing.T) {
	fake := &fakeMemberService{status: http.StatusNotFound}
	accounts := NewLiveAccounts(newTestClient(t, fake))
	assert.False(t, accounts.IsLive(context.Background(), liveTestSFID))
	assert.WithinDuration(t, time.Now().Add(notLiveAccountTTL), accounts.answers[liveTestSFID].expires, 10*time.Second)

	accounts.answers[liveTestSFID] = liveAnswer{expires: time.Now().Add(-time.Second)}
	fake.status, fake.response = http.StatusOK, liveTestOrg
	assert.True(t, accounts.IsLive(context.Background(), liveTestSFID))
	assert.Equal(t, 2, fake.getCalls)
	assert.WithinDuration(t, time.Now().Add(liveAccountTTL), accounts.answers[liveTestSFID].expires, 10*time.Second)
}

func TestLiveAccountsWithoutAClientOrAnAccountID(t *testing.T) {
	assert.False(t, NewLiveAccounts(nil).IsLive(context.Background(), liveTestSFID))
	var none *LiveAccounts
	assert.False(t, none.IsLive(context.Background(), liveTestSFID))

	fake := &fakeMemberService{status: http.StatusOK, response: liveTestOrg}
	accounts := NewLiveAccounts(newTestClient(t, fake))
	assert.False(t, accounts.IsLive(context.Background(), "lf-not-an-sfid"))
	assert.Zero(t, fake.getCalls+fake.tokenCalls, "nothing is asked for an id that is not a Salesforce id")
}

func TestLiveAccountsWhenNoTokenIsIssued(t *testing.T) {
	fake := &fakeMemberService{status: http.StatusOK, response: liveTestOrg, tokenStatus: http.StatusForbidden}
	accounts := NewLiveAccounts(newTestClient(t, fake))
	assert.False(t, accounts.IsLive(context.Background(), liveTestSFID))
	assert.Zero(t, fake.getCalls)
}

func TestLiveAccountsShareOneLookupThatNoCallerCutsShort(t *testing.T) {
	var mu sync.Mutex
	lookups := 0
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == claAdminsOAuthPath {
			if _, err := w.Write([]byte(`{"access_token":"member-token","token_type":"Bearer","expires_in":3600}`)); err != nil {
				t.Error(err)
			}
			return
		}
		mu.Lock()
		lookups++
		mu.Unlock()
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		if _, err := w.Write([]byte(`{"uid":"` + liveTestSFID + `","name":"Example Org"}`)); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	client, err := NewClient(Config{BaseURL: server.URL, Audience: "https://member.example/", OAuthTokenURL: server.URL + claAdminsOAuthPath, ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	accounts := NewLiveAccounts(client)

	cancelled, cancel := context.WithCancel(context.Background())
	done := make(chan bool)
	go func() { done <- accounts.IsLive(cancelled, liveTestSFID) }()
	waitFor(t, func() bool { mu.Lock(); defer mu.Unlock(); return lookups == 1 })
	cancel()
	assert.False(t, <-done, "a caller that gave up gets no lens address, without waiting for the answer")

	const callers = 8
	answers := make(chan bool, callers)
	for i := 0; i < callers; i++ {
		go func() { answers <- accounts.IsLive(context.Background(), liveTestSFID) }()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	for i := 0; i < callers; i++ {
		assert.True(t, <-answers, "the request the first caller started goes on for the others")
	}
	live, kept := accounts.cached(liveTestSFID)
	assert.True(t, kept && live, "its answer is kept")
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, lookups, "the callers that overlap share one request")
}

func TestLiveAccountsSurviveAPanicOfTheLookup(t *testing.T) {
	accounts := NewLiveAccounts(&Client{cfg: Config{BaseURL: "http://member.invalid", OAuthTokenURL: "http://member.invalid/oauth/token"}})

	assert.False(t, accounts.IsLive(context.Background(), liveTestSFID), "a lookup that panics (no HTTP client) is \"not live\"")
	_, kept := accounts.cached(liveTestSFID)
	assert.False(t, kept, "and nothing is kept")
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
