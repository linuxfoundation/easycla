// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type auth0Result struct {
	identities []Auth0Identity
	err        error
}

// fakeAuth0Inner returns its programmed results in turn per LF username, repeating the last one
type fakeAuth0Inner struct {
	mu      sync.Mutex
	results map[string][]auth0Result
	calls   map[string]int
	block   chan struct{}
}

func newFakeAuth0Inner() *fakeAuth0Inner {
	return &fakeAuth0Inner{results: map[string][]auth0Result{}, calls: map[string]int{}}
}

func (f *fakeAuth0Inner) program(lfUsername string, results ...auth0Result) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.results[lfUsername] = append(f.results[lfUsername], results...)
}

func (f *fakeAuth0Inner) callCount(lfUsername string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[lfUsername]
}

func (f *fakeAuth0Inner) UserIdentities(_ context.Context, lfUsername string) ([]Auth0Identity, error) {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	results := f.results[lfUsername]
	result := results[min(f.calls[lfUsername], len(results)-1)]
	f.calls[lfUsername]++
	return result.identities, result.err
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestAuth0Cache(inner Auth0IdentityService) (*cachedAuth0Identities, *fakeClock) {
	clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
	cache := newCachedAuth0Identities(inner)
	cache.now = clock.Now
	return cache, clock
}

var (
	gitlabIdentity = []Auth0Identity{{Provider: "gitlab", UserID: "777", Username: "someone-gl"}}
	rateLimited    = fmt.Errorf("%w: status 429 after 3 attempts: Global limit has been reached", errAuth0RateLimited)
)

func TestCachedAuth0IdentitiesFreshHit(t *testing.T) {
	inner := newFakeAuth0Inner()
	inner.program("someone", auth0Result{identities: gitlabIdentity})
	cache, clock := newTestAuth0Cache(inner)

	identities, err := cache.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)
	assert.Equal(t, gitlabIdentity, identities)

	clock.Advance(4 * time.Minute)
	identities, err = cache.UserIdentities(context.Background(), " someone ")
	require.NoError(t, err)
	assert.Equal(t, gitlabIdentity, identities)
	assert.Equal(t, 1, inner.callCount("someone"))

	identities, err = cache.UserIdentities(context.Background(), "")
	require.NoError(t, err)
	assert.Nil(t, identities)
}

func TestCachedAuth0IdentitiesExpiry(t *testing.T) {
	inner := newFakeAuth0Inner()
	inner.program("someone", auth0Result{identities: gitlabIdentity}, auth0Result{identities: nil})
	cache, clock := newTestAuth0Cache(inner)

	_, err := cache.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)

	clock.Advance(auth0IdentitiesCacheTTL + time.Second)
	identities, err := cache.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)
	assert.Empty(t, identities, "an expired entry must be refreshed, picking up an unlinked account")
	assert.Equal(t, 2, inner.callCount("someone"))
}

func TestCachedAuth0IdentitiesStaleOnRateLimit(t *testing.T) {
	inner := newFakeAuth0Inner()
	inner.program("someone", auth0Result{identities: gitlabIdentity}, auth0Result{err: rateLimited})
	cache, clock := newTestAuth0Cache(inner)

	_, err := cache.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)

	clock.Advance(30 * time.Minute)
	identities, err := cache.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)
	assert.Equal(t, gitlabIdentity, identities)
	assert.Equal(t, 2, inner.callCount("someone"))
}

func TestCachedAuth0IdentitiesNoStaleBeyondWindow(t *testing.T) {
	inner := newFakeAuth0Inner()
	inner.program("someone", auth0Result{identities: gitlabIdentity}, auth0Result{err: rateLimited})
	cache, clock := newTestAuth0Cache(inner)

	_, err := cache.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)

	clock.Advance(61 * time.Minute)
	identities, err := cache.UserIdentities(context.Background(), "someone")
	require.ErrorIs(t, err, errAuth0RateLimited)
	assert.Nil(t, identities)
}

func TestCachedAuth0IdentitiesNoStaleForOtherErrors(t *testing.T) {
	inner := newFakeAuth0Inner()
	forbidden := errors.New("auth0 management api returned status 403: insufficient scope")
	inner.program("someone", auth0Result{identities: gitlabIdentity}, auth0Result{err: forbidden})
	cache, clock := newTestAuth0Cache(inner)

	_, err := cache.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)

	clock.Advance(6 * time.Minute)
	identities, err := cache.UserIdentities(context.Background(), "someone")
	require.ErrorIs(t, err, forbidden)
	assert.Nil(t, identities)
}

func TestCachedAuth0IdentitiesNoEntryRateLimited(t *testing.T) {
	inner := newFakeAuth0Inner()
	inner.program("someone", auth0Result{err: rateLimited})
	cache, _ := newTestAuth0Cache(inner)

	identities, err := cache.UserIdentities(context.Background(), "someone")
	require.ErrorIs(t, err, errAuth0RateLimited)
	assert.Nil(t, identities)

	// through the service, the rate-limited lookup skips the keys rather than allowing them
	svc := newTestService(&fakeRepo{}, nil, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})
	svc.auth0Identities = cache
	allowed, skipped, err := svc.AuthorizeIdentity(context.Background(), &Caller{Username: "someone"}, &Identity{
		GitlabIDs:       []int64{777},
		GitlabUsernames: []string{"someone-gl"},
	})
	require.NoError(t, err)
	assert.Empty(t, allowed.GitlabIDs)
	assert.Empty(t, allowed.GitlabUsernames)
	assert.Equal(t, []string{"gitlabId:777", "gitlabUsername:someone-gl"}, skipped)
}

func TestCachedAuth0IdentitiesPerUserIsolation(t *testing.T) {
	inner := newFakeAuth0Inner()
	inner.program("user-a", auth0Result{identities: gitlabIdentity})
	inner.program("user-b", auth0Result{err: rateLimited})
	cache, _ := newTestAuth0Cache(inner)

	_, err := cache.UserIdentities(context.Background(), "user-a")
	require.NoError(t, err)

	identities, err := cache.UserIdentities(context.Background(), "user-b")
	require.ErrorIs(t, err, errAuth0RateLimited)
	assert.Nil(t, identities, "another user's cached identities must never stand in")
	assert.Equal(t, 1, inner.callCount("user-a"))
	assert.Equal(t, 1, inner.callCount("user-b"))
}

func TestCachedAuth0IdentitiesSingleflight(t *testing.T) {
	inner := newFakeAuth0Inner()
	inner.program("someone", auth0Result{identities: gitlabIdentity})
	inner.block = make(chan struct{})
	cache, _ := newTestAuth0Cache(inner)

	const callers = 8
	var started, done sync.WaitGroup
	results := make([][]Auth0Identity, callers)
	errs := make([]error, callers)
	for i := range callers {
		started.Add(1)
		done.Add(1)
		go func(i int) {
			defer done.Done()
			started.Done()
			results[i], errs[i] = cache.UserIdentities(context.Background(), "someone")
		}(i)
	}
	started.Wait()
	// give every caller time to join the in-flight lookup before releasing it
	time.Sleep(50 * time.Millisecond)
	close(inner.block)
	done.Wait()

	assert.Equal(t, 1, inner.callCount("someone"))
	for i := range callers {
		require.NoError(t, errs[i])
		assert.Equal(t, gitlabIdentity, results[i])
	}
}

func TestCachedAuth0IdentitiesCallerCancelled(t *testing.T) {
	inner := newFakeAuth0Inner()
	inner.program("someone", auth0Result{identities: gitlabIdentity})
	inner.block = make(chan struct{})
	cache, _ := newTestAuth0Cache(inner)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := cache.UserIdentities(ctx, "someone")
	require.ErrorIs(t, err, context.Canceled)

	// the detached lookup still completes and fills the cache for the next caller
	close(inner.block)
	identities, err := cache.UserIdentities(context.Background(), "someone")
	require.NoError(t, err)
	assert.Equal(t, gitlabIdentity, identities)
	assert.Equal(t, 1, inner.callCount("someone"))
}

func TestCachedAuth0IdentitiesBounded(t *testing.T) {
	inner := newFakeAuth0Inner()
	for _, user := range []string{"user-a", "user-b", "user-c"} {
		inner.program(user, auth0Result{identities: gitlabIdentity})
	}
	cache, clock := newTestAuth0Cache(inner)
	cache.maxEntries = 2

	for _, user := range []string{"user-a", "user-b", "user-c"} {
		_, err := cache.UserIdentities(context.Background(), user)
		require.NoError(t, err)
		clock.Advance(time.Second)
	}

	assert.Len(t, cache.entries, 2)
	assert.NotContains(t, cache.entries, "user-a", "the oldest lookup must be evicted")
	assert.Contains(t, cache.entries, "user-b")
	assert.Contains(t, cache.entries, "user-c")
}
