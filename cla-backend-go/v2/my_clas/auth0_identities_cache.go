// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
)

const (
	// auth0IdentitiesCacheTTL is how long a successful lookup is served without asking Auth0 -
	// linking or unlinking an account takes effect in My CLAs within this window
	auth0IdentitiesCacheTTL = 5 * time.Minute
	// auth0IdentitiesStaleTTL is how old a lookup may be and still stand in while Auth0 keeps
	// rate-limiting the refresh
	auth0IdentitiesStaleTTL = time.Hour
	// auth0IdentitiesCacheMaxEntries bounds the per-container cache
	auth0IdentitiesCacheMaxEntries = 10000
	// auth0LookupTimeout bounds a lookup that runs detached from the request that triggered it:
	// the 30s HTTP client timeout plus the retry budget plus a margin
	auth0LookupTimeout = 35 * time.Second
)

type auth0CacheEntry struct {
	identities []Auth0Identity
	fetchedAt  time.Time
}

// cachedAuth0Identities caches each LF username's Auth0 identities in memory. Concurrent misses
// for the same user share one lookup, and while Auth0 keeps rate-limiting the user's last
// successful lookup stands in for up to staleTTL. Any other failure is returned unchanged. The
// returned slices are shared with every caller and must be treated as read-only.
type cachedAuth0Identities struct {
	inner      Auth0IdentityService
	ttl        time.Duration
	staleTTL   time.Duration
	maxEntries int
	now        func() time.Time
	flight     singleflight.Group

	mu      sync.Mutex
	entries map[string]auth0CacheEntry
}

// newCachedAuth0Identities wraps a live client with bounded per-user history caching.
func newCachedAuth0Identities(inner Auth0IdentityService) *cachedAuth0Identities {
	return &cachedAuth0Identities{
		inner:      inner,
		ttl:        auth0IdentitiesCacheTTL,
		staleTTL:   auth0IdentitiesStaleTTL,
		maxEntries: auth0IdentitiesCacheMaxEntries,
		now:        time.Now,
		entries:    make(map[string]auth0CacheEntry),
	}
}

// UserIdentities serves fresh history evidence or shares a detached refresh. Only rate-limit
// failures may reuse stale results; this method must not authorize a new signing session.
func (c *cachedAuth0Identities) UserIdentities(ctx context.Context, lfUsername string) ([]Auth0Identity, error) {
	lfUsername = strings.TrimSpace(lfUsername)
	if lfUsername == "" {
		return nil, nil
	}
	if identities, ok := c.fresh(lfUsername); ok {
		return identities, nil
	}

	f := logrus.Fields{
		"functionName":   "v2.my_clas.cachedAuth0Identities.UserIdentities",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"lfUsername":     lfUsername,
	}
	fill := c.flight.DoChan(lfUsername, func() (interface{}, error) {
		// a lookup that finished while this call waited makes this one unnecessary
		if identities, ok := c.fresh(lfUsername); ok {
			return identities, nil
		}
		// the lookup outlives the request that happened to trigger it - one caller giving up must
		// not fail the lookup for every other caller waiting on it
		lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), auth0LookupTimeout)
		defer cancel()
		identities, lookupErr := c.inner.UserIdentities(lookupCtx, lfUsername)
		if lookupErr != nil {
			return nil, lookupErr
		}
		c.store(lfUsername, identities)
		log.WithFields(f).Debugf("cache miss - loaded %d Auth0 identities", len(identities))
		return identities, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-fill:
		if result.Err == nil {
			identities, _ := result.Val.([]Auth0Identity) // nolint:errcheck
			return identities, nil
		}
		if errors.Is(result.Err, errAuth0RateLimited) {
			if entry, ok := c.entry(lfUsername); ok {
				if age := c.now().Sub(entry.fetchedAt); age < c.staleTTL {
					log.WithFields(f).WithField("ageSeconds", int64(age.Seconds())).WithError(result.Err).
						Warn("serving stale Auth0 identities after rate limiting")
					return entry.identities, nil
				}
			}
		}
		return nil, result.Err
	}
}

// entry reads a cached result under the mutex without changing its original age.
func (c *cachedAuth0Identities) entry(lfUsername string) (auth0CacheEntry, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[lfUsername]
	return entry, ok
}

// fresh returns a cached result only while it is younger than the normal cache TTL.
func (c *cachedAuth0Identities) fresh(lfUsername string) ([]Auth0Identity, bool) {
	entry, ok := c.entry(lfUsername)
	if !ok || c.now().Sub(entry.fetchedAt) >= c.ttl {
		return nil, false
	}
	return entry.identities, true
}

// store records a successful lookup. A new user arriving at a full cache first prunes entries too
// old to serve even as stale, then evicts the oldest lookup if it is still full.
func (c *cachedAuth0Identities) store(lfUsername string, identities []Auth0Identity) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if _, exists := c.entries[lfUsername]; !exists && len(c.entries) >= c.maxEntries {
		for key, entry := range c.entries {
			if now.Sub(entry.fetchedAt) >= c.staleTTL {
				delete(c.entries, key)
			}
		}
		if len(c.entries) >= c.maxEntries {
			var oldestKey string
			var oldest time.Time
			for key, entry := range c.entries {
				if oldestKey == "" || entry.fetchedAt.Before(oldest) {
					oldestKey, oldest = key, entry.fetchedAt
				}
			}
			delete(c.entries, oldestKey)
		}
	}
	c.entries[lfUsername] = auth0CacheEntry{identities: identities, fetchedAt: now}
}
