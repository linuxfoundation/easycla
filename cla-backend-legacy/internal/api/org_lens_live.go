// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/config"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/legacy/userservice"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/logging"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/store"
)

const (
	orgLensLiveTTL     = 10 * time.Minute
	orgLensNotLiveTTL  = time.Minute
	orgLensLiveTimeout = 5 * time.Second
)

type orgLensAnswer struct {
	live    bool
	expires time.Time
}

// orgLensAccounts answers whether a Salesforce Account is live, that is member-service GET /b2b_orgs/{uid} answers 200
type orgLensAccounts struct {
	mu         sync.Mutex
	base       string
	tokens     *userservice.Client
	httpClient *http.Client
	answers    map[string]orgLensAnswer
}

var orgLensLiveAccounts = &orgLensAccounts{}

func (a *orgLensAccounts) client(ctx context.Context) (string, *userservice.Client, *http.Client, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.httpClient == nil {
		a.httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	if a.tokens != nil {
		return a.base, a.tokens, a.httpClient, nil
	}
	stage := store.StageFromEnv()
	base, err := config.GetSSMParameter(ctx, "cla-member-service-base-url-"+stage)
	if err != nil {
		return "", nil, nil, err
	}
	audience, err := config.GetSSMParameter(ctx, "cla-member-service-auth0-audience-"+stage)
	if err != nil {
		return "", nil, nil, err
	}
	a.base, a.tokens = strings.TrimRight(strings.TrimSpace(base), "/"), userservice.NewForAudience(a.httpClient, audience)
	return a.base, a.tokens, a.httpClient, nil
}

func (a *orgLensAccounts) ask(ctx context.Context, companySFID string) (bool, error) {
	base, tokens, httpClient, err := a.client(ctx)
	if err != nil {
		return false, err
	}
	token, err := tokens.AccessToken(ctx)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/b2b_orgs/"+url.PathEscape(companySFID)+"?v=1", nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, err
	}
	_ = resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusForbidden, http.StatusNotFound:
		return false, nil
	}
	return false, fmt.Errorf("member-service answered %d", resp.StatusCode)
}

func (a *orgLensAccounts) live(ctx context.Context, companySFID string) bool {
	a.mu.Lock()
	answer, known := a.answers[companySFID]
	a.mu.Unlock()
	if known && time.Now().Before(answer.expires) {
		return answer.live
	}

	ctx, cancel := context.WithTimeout(ctx, orgLensLiveTimeout)
	defer cancel()
	live, err := a.ask(ctx, companySFID)
	if err != nil {
		logging.Warnf("unable to check that the Account %s is live, this e-mail keeps the Corporate Console link: %v", companySFID, err)
	}
	answer = orgLensAnswer{live: live, expires: time.Now().Add(orgLensNotLiveTTL)}
	if live {
		answer.expires = time.Now().Add(orgLensLiveTTL)
	}
	a.mu.Lock()
	if a.answers == nil {
		a.answers = map[string]orgLensAnswer{}
	}
	a.answers[companySFID] = answer
	a.mu.Unlock()
	return live
}

// lensURL is orgLensURL for a company whose Salesforce Account is live
func (a *orgLensAccounts) lensURL(ctx context.Context, base, companySFID, claGroupID string) string {
	lensURL := orgLensURL(base, companySFID, claGroupID)
	if lensURL == "" || !a.live(ctx, strings.TrimSpace(companySFID)) {
		return ""
	}
	return lensURL
}
