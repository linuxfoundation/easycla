// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package member_service

import (
	"net/http"
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
			assert.Equal(t, tt.live, accounts.IsLive(liveTestSFID))
			assert.Equal(t, tt.live, accounts.IsLive(" 0014100000Te0G7 "), "the 15-character form names the same Account")
			assert.Equal(t, 1, fake.getCalls, "the answer is kept")
			assert.Equal(t, []string{"/b2b_orgs/" + liveTestSFID}, fake.getPaths)
		})
	}
}

func TestLiveAccountsAskAgainWhenTheAnswerIsOld(t *testing.T) {
	fake := &fakeMemberService{status: http.StatusNotFound}
	accounts := NewLiveAccounts(newTestClient(t, fake))
	assert.False(t, accounts.IsLive(liveTestSFID))
	assert.WithinDuration(t, time.Now().Add(notLiveAccountTTL), accounts.answers[liveTestSFID].expires, 10*time.Second)

	accounts.answers[liveTestSFID] = liveAnswer{expires: time.Now().Add(-time.Second)}
	fake.status, fake.response = http.StatusOK, liveTestOrg
	assert.True(t, accounts.IsLive(liveTestSFID))
	assert.Equal(t, 2, fake.getCalls)
	assert.WithinDuration(t, time.Now().Add(liveAccountTTL), accounts.answers[liveTestSFID].expires, 10*time.Second)
}

func TestLiveAccountsWithoutAClientOrAnAccountID(t *testing.T) {
	assert.False(t, NewLiveAccounts(nil).IsLive(liveTestSFID))
	var none *LiveAccounts
	assert.False(t, none.IsLive(liveTestSFID))

	fake := &fakeMemberService{status: http.StatusOK, response: liveTestOrg}
	accounts := NewLiveAccounts(newTestClient(t, fake))
	assert.False(t, accounts.IsLive("lf-not-an-sfid"))
	assert.Zero(t, fake.getCalls+fake.tokenCalls, "nothing is asked for an id that is not a Salesforce id")
}

func TestLiveAccountsWhenNoTokenIsIssued(t *testing.T) {
	fake := &fakeMemberService{status: http.StatusOK, response: liveTestOrg, tokenStatus: http.StatusForbidden}
	accounts := NewLiveAccounts(newTestClient(t, fake))
	assert.False(t, accounts.IsLive(liveTestSFID))
	assert.Zero(t, fake.getCalls)
}
