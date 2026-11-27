// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package member_service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"golang.org/x/sync/singleflight"
)

const (
	liveAccountTTL     = 10 * time.Minute
	notLiveAccountTTL  = time.Minute
	liveAccountTimeout = 5 * time.Second
)

type liveAnswer struct {
	live    bool
	expires time.Time
}

// LiveAccounts answers whether a Salesforce Account is live, that is GET /b2b_orgs/{uid} answers 200, and keeps each answer for a short time
type LiveAccounts struct {
	client  *Client
	mu      sync.Mutex
	answers map[string]liveAnswer
	lookups singleflight.Group
}

// NewLiveAccounts returns the check; without a client no Account is live
func NewLiveAccounts(client *Client) *LiveAccounts {
	return &LiveAccounts{client: client, answers: map[string]liveAnswer{}}
}

// IsLive reports whether the Account is live; any other answer than 200, and no answer, is "not live".
// Lookups of one Account that overlap share one request, which no caller can cut short: a caller whose
// context ends gets "not live" at once and the request goes on for the others.
func (l *LiveAccounts) IsLive(ctx context.Context, sfid string) bool {
	if l == nil || l.client == nil {
		return false
	}
	uid, ok := sfid18(sfid)
	if !ok {
		return false
	}
	if answer, known := l.cached(uid); known {
		return answer
	}
	shared := l.lookups.DoChan(uid, func() (interface{}, error) {
		if answer, known := l.cached(uid); known {
			return answer, nil
		}
		callCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), liveAccountTimeout)
		defer cancel()
		_, err := l.client.GetB2BOrg(callCtx, uid)
		answer := liveAnswer{live: err == nil, expires: time.Now().Add(notLiveAccountTTL)}
		var authErr *AuthError
		switch {
		case err == nil:
			answer.expires = time.Now().Add(liveAccountTTL)
		case errors.Is(err, ErrOrgNotFound), errors.As(err, &authErr) && authErr.Status == http.StatusForbidden && !authErr.Token:
		default:
			log.WithError(err).Warnf("unable to check that the Account %s is live, it gets no Organization lens address for now", uid)
		}
		l.mu.Lock()
		l.answers[uid] = answer
		l.mu.Unlock()
		return answer.live, nil
	})
	select {
	case result := <-shared:
		live, ok := result.Val.(bool)
		return result.Err == nil && ok && live
	case <-ctx.Done():
		return false
	}
}

func (l *LiveAccounts) cached(uid string) (live, known bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	answer, ok := l.answers[uid]
	return answer.live, ok && time.Now().Before(answer.expires)
}
