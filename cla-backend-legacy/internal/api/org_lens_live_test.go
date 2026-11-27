// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/legacy/userservice"
)

const (
	orgLensLiveTestSFID  = "001000000000000AAA"
	orgLensLiveTestBase  = "https://app.example.org"
	orgLensLiveTestGroup = "00000000-0000-4000-8000-000000000001"
	orgLensLiveTestPage  = orgLensLiveTestBase + "/org/" + orgLensLiveTestSFID + "/easycla/" + orgLensLiveTestGroup
)

type orgLensFakeMemberService struct {
	t           *testing.T
	status      int
	tokenStatus int
	tokenCalls  int
	getCalls    int
}

func (f *orgLensFakeMemberService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/oauth/token":
		f.tokenCalls++
		if err := r.ParseForm(); err != nil {
			f.t.Error(err)
		}
		if got := r.PostForm.Get("audience"); got != "https://member.example/" {
			f.t.Errorf("token audience = %q", got)
		}
		if f.tokenStatus != 0 {
			w.WriteHeader(f.tokenStatus)
			return
		}
		_, _ = w.Write([]byte(`{"access_token":"member-token","token_type":"Bearer","expires_in":3600}`))
	case "/b2b_orgs/" + orgLensLiveTestSFID:
		f.getCalls++
		if r.Method != http.MethodGet || r.Header.Get("Authorization") != "Bearer member-token" || r.URL.Query().Get("v") != "1" {
			f.t.Errorf("unexpected request %s %s authorization=%q", r.Method, r.URL, r.Header.Get("Authorization"))
		}
		w.WriteHeader(f.status)
	default:
		f.t.Errorf("unexpected path %s", r.URL.Path)
		http.NotFound(w, r)
	}
}

func orgLensTestAccounts(t *testing.T, fake *orgLensFakeMemberService) *orgLensAccounts {
	fake.t = t
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	t.Setenv("PLATFORM_AUTH0_URL", server.URL+"/oauth/token")
	t.Setenv("PLATFORM_AUTH0_CLIENT_ID", "id")
	t.Setenv("PLATFORM_AUTH0_CLIENT_SECRET", "secret")
	t.Setenv("PLATFORM_AUTH0_AUDIENCE", "https://platform.example/")
	return &orgLensAccounts{base: server.URL, tokens: userservice.NewForAudience(server.Client(), "https://member.example/")}
}

func TestOrgLensLinkNeedsALiveAccount(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		tokenStatus int
		want        string
		wantGets    int
	}{
		{"200 is live", http.StatusOK, 0, orgLensLiveTestPage, 1},
		{"403 keeps the Corporate Console", http.StatusForbidden, 0, "", 1},
		{"404 keeps the Corporate Console", http.StatusNotFound, 0, "", 1},
		{"500 keeps the Corporate Console", http.StatusInternalServerError, 0, "", 1},
		{"no token keeps the Corporate Console", http.StatusOK, http.StatusForbidden, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &orgLensFakeMemberService{status: tt.status, tokenStatus: tt.tokenStatus}
			accounts := orgLensTestAccounts(t, fake)
			for i := 0; i < 2; i++ {
				if got := accounts.lensURL(context.Background(), orgLensLiveTestBase, " "+orgLensLiveTestSFID+" ", orgLensLiveTestGroup); got != tt.want {
					t.Errorf("lensURL() = %q, want %q", got, tt.want)
				}
			}
			if fake.getCalls != tt.wantGets || fake.tokenCalls != 1 {
				t.Errorf("the answer is kept: Account lookups = %d, want %d; token requests = %d, want 1", fake.getCalls, tt.wantGets, fake.tokenCalls)
			}
		})
	}
}

func TestOrgLensLinkAsksAgainWhenTheAnswerIsOld(t *testing.T) {
	fake := &orgLensFakeMemberService{status: http.StatusNotFound}
	accounts := orgLensTestAccounts(t, fake)
	if accounts.live(context.Background(), orgLensLiveTestSFID) {
		t.Fatal("404 must not be live")
	}
	accounts.answers[orgLensLiveTestSFID] = orgLensAnswer{expires: time.Now().Add(-time.Second)}
	fake.status = http.StatusOK
	if !accounts.live(context.Background(), orgLensLiveTestSFID) || fake.getCalls != 2 {
		t.Errorf("an old answer is asked again: Account lookups = %d, want 2", fake.getCalls)
	}
}

func TestOrgLensLinkAsksNothingWithoutABaseOrAnAccountID(t *testing.T) {
	fake := &orgLensFakeMemberService{status: http.StatusOK}
	accounts := orgLensTestAccounts(t, fake)
	for _, tt := range [][2]string{{"", orgLensLiveTestSFID}, {orgLensLiveTestBase, "lf0000000000000AAA"}, {orgLensLiveTestBase, ""}} {
		if got := accounts.lensURL(context.Background(), tt[0], tt[1], orgLensLiveTestGroup); got != "" {
			t.Errorf("lensURL(%q, %q) = %q, want none", tt[0], tt[1], got)
		}
	}
	if fake.getCalls+fake.tokenCalls != 0 {
		t.Errorf("member-service must not be asked: lookups = %d, token requests = %d", fake.getCalls, fake.tokenCalls)
	}
}

func TestOrgLensSettingsAreReadOutsideTheAnswersLockAndAnAbsentOneIsKept(t *testing.T) {
	reads := 0
	accounts := &orgLensAccounts{}
	accounts.readSSM = func(context.Context, string) (string, error) {
		reads++
		if !accounts.mu.TryLock() {
			t.Error("the settings are read while the lock of the answers is held")
			return "", errors.New("locked")
		}
		accounts.mu.Unlock()
		return "", &ssmtypes.ParameterNotFound{}
	}
	for i := 0; i < 3; i++ {
		if got := accounts.lensURL(context.Background(), orgLensLiveTestBase, orgLensLiveTestSFID, orgLensLiveTestGroup); got != "" {
			t.Errorf("lensURL() = %q, want none without member-service settings", got)
		}
		delete(accounts.answers, orgLensLiveTestSFID)
	}
	if reads != 1 {
		t.Errorf("an absent setting is read once per process: %d reads", reads)
	}

	unreadable := &orgLensAccounts{}
	unreadable.readSSM = func(context.Context, string) (string, error) {
		reads++
		return "", errors.New("throttled")
	}
	reads = 0
	for i := 0; i < 2; i++ {
		if unreadable.live(context.Background(), orgLensLiveTestSFID) {
			t.Error("an unreadable setting must not be live")
		}
		delete(unreadable.answers, orgLensLiveTestSFID)
	}
	if reads != 2 {
		t.Errorf("an unreadable setting is read again: %d reads", reads)
	}
}

func TestOrgLensLinkDoesNotKeepAnAnswerOfACallerThatGaveUp(t *testing.T) {
	fake := &orgLensFakeMemberService{status: http.StatusOK}
	accounts := orgLensTestAccounts(t, fake)
	gone, cancel := context.WithCancel(context.Background())
	cancel()
	if accounts.live(gone, orgLensLiveTestSFID) {
		t.Fatal("a caller that gave up gets no lens link")
	}
	if _, kept := accounts.answers[orgLensLiveTestSFID]; kept {
		t.Error("its answer must not be kept")
	}
	if !accounts.live(context.Background(), orgLensLiveTestSFID) {
		t.Error("the next caller asks again and gets the live answer")
	}
}

func TestOrgLensLinkUsesTheCanonicalAccountID(t *testing.T) {
	fake := &orgLensFakeMemberService{status: http.StatusOK}
	accounts := orgLensTestAccounts(t, fake)
	if got := accounts.lensURL(context.Background(), orgLensLiveTestBase, "001000000000000aaa", orgLensLiveTestGroup); got != orgLensLiveTestPage {
		t.Errorf("lensURL() = %q, want %q", got, orgLensLiveTestPage)
	}
	if fake.getCalls != 1 {
		t.Errorf("Account lookups = %d, want 1 with the canonical id", fake.getCalls)
	}
	for stored, want := range map[string]string{
		"0014100000Te0G7AAJ": "0014100000Te0G7AAJ",
		"0014100000te0g7aaj": "0014100000Te0G7AAJ",
		"0014100000TE0G7AAJ": "0014100000Te0G7AAJ",
		"0014100000Te0G7":    "0014100000Te0G7",
		"0014100000Te0G7AA9": "0014100000Te0G7AA9",
		"0014100000Te0G7AAZ": "0014100000Te0G7AAZ",
		"0014ı0000000000AA":  "0014ı0000000000AA",
		"0014100000Te0G7ſA":  "0014100000Te0G7ſA",
		"0014100000Te0G7/AJ": "0014100000Te0G7/AJ",
	} {
		if got := canonicalAccountID(stored); got != want {
			t.Errorf("canonicalAccountID(%q) = %q, want %q", stored, got, want)
		}
	}
	if got := accounts.lensURL(context.Background(), orgLensLiveTestBase, "0014ı0000000000AA", orgLensLiveTestGroup); got != "" {
		t.Errorf("an id with a character outside the Salesforce alphabet gets no lens link, got %q", got)
	}
}
