// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

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

// errOrgLensNoMemberService means the member-service settings are absent in this stage: no Account is live
var errOrgLensNoMemberService = errors.New("member-service is not configured")

// orgLensAccounts answers whether a Salesforce Account is live, that is member-service GET /b2b_orgs/{uid} answers 200.
// mu guards only the answers; the settings have their own lock so that a slow read never holds up a kept answer.
type orgLensAccounts struct {
	mu      sync.Mutex
	answers map[string]orgLensAnswer

	settingsMu sync.Mutex
	absent     bool
	base       string
	tokens     *userservice.Client
	httpClient *http.Client
	readSSM    func(ctx context.Context, name string) (string, error)
}

var orgLensLiveAccounts = &orgLensAccounts{}

// client reads the two member-service settings once; an absent setting is kept for the process, an unreadable one is read again
func (a *orgLensAccounts) client(ctx context.Context) (string, *userservice.Client, *http.Client, error) {
	a.settingsMu.Lock()
	defer a.settingsMu.Unlock()
	if a.httpClient == nil {
		a.httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	if a.tokens != nil {
		return a.base, a.tokens, a.httpClient, nil
	}
	if a.absent {
		return "", nil, nil, errOrgLensNoMemberService
	}
	read := a.readSSM
	if read == nil {
		read = config.GetSSMParameter
	}
	stage := store.StageFromEnv()
	values := make([]string, 0, 2)
	for _, name := range []string{"cla-member-service-base-url-" + stage, "cla-member-service-auth0-audience-" + stage} {
		value, err := read(ctx, name)
		var notFound *ssmtypes.ParameterNotFound
		if errors.As(err, &notFound) {
			a.absent = true
			logging.Warnf("%s is not set: e-mails keep the Corporate Console link", name)
			return "", nil, nil, errOrgLensNoMemberService
		}
		if err != nil {
			return "", nil, nil, err
		}
		values = append(values, value)
	}
	a.base, a.tokens = strings.TrimRight(strings.TrimSpace(values[0]), "/"), userservice.NewForAudience(a.httpClient, values[1])
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
	if err != nil && !errors.Is(err, errOrgLensNoMemberService) {
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
