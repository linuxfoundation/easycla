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

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
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
	started chan context.Context
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

func (f *fakeAuth0Inner) UserIdentities(ctx context.Context, lfUsername string) ([]Auth0Identity, error) {
	f.mu.Lock()
	results := f.results[lfUsername]
	result := results[min(f.calls[lfUsername], len(results)-1)]
	f.calls[lfUsername]++
	f.mu.Unlock()
	if f.started != nil {
		f.started <- ctx
	}
	if f.block != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-f.block:
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result.identities, result.err
}

// Done is evaluated after the caller joins the singleflight, so this signals an
// attached waiter without relying on scheduler timing or sleeps.
type auth0WaiterContext struct {
	context.Context
	waiting chan struct{}
	once    sync.Once
}

func (c *auth0WaiterContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
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

func TestAuthorizeIdentityBypassesAuth0Cache(t *testing.T) {
	for _, tc := range []struct {
		name       string
		age        time.Duration
		initial    []Auth0Identity
		live       auth0Result
		wantLinked bool
	}{
		{name: "fresh cached link removed", age: time.Minute, initial: gitlabIdentity},
		{name: "stale cached link removed", age: 30 * time.Minute, initial: gitlabIdentity},
		{name: "fresh cached link rate limited", age: time.Minute, initial: gitlabIdentity, live: auth0Result{err: rateLimited}},
		{name: "stale cached link rate limited", age: 30 * time.Minute, initial: gitlabIdentity, live: auth0Result{err: rateLimited}},
		{name: "new link bypasses cached empty result", age: time.Minute, live: auth0Result{identities: gitlabIdentity}, wantLinked: true},
		{name: "current link still verified", age: time.Minute, initial: gitlabIdentity, live: auth0Result{identities: gitlabIdentity}, wantLinked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inner := newFakeAuth0Inner()
			inner.program("someone", auth0Result{identities: tc.initial}, tc.live)
			cache, clock := newTestAuth0Cache(inner)
			svc := newTestService(&fakeRepo{}, nil, &fakeSignatures{}, &fakeCompanies{}, &fakeClaGroups{})
			svc.auth0Identities = cache
			caller := &Caller{Username: "someone"}
			requested := &Identity{GitlabIDs: []int64{777}, GitlabUsernames: []string{"someone-gl"}}
			skippedKeys := []string{"gitlabId:777", "gitlabUsername:someone-gl"}
			ctx := context.Background()

			// A history read fills the same cache used by the production service.
			history, err := svc.GetMyClas(ctx, caller, requested, nil, nil)
			require.NoError(t, err)
			if len(tc.initial) > 0 {
				assert.Empty(t, history.SkippedIdentities)
			} else {
				assert.Equal(t, skippedKeys, history.SkippedIdentities)
			}
			require.Equal(t, 1, inner.callCount("someone"))
			clock.Advance(tc.age)

			// Signing's ownership check must use the live result, even with eligible cached evidence.
			allowed, skipped, err := svc.AuthorizeIdentity(ctx, caller, requested)
			require.NoError(t, err)
			if tc.wantLinked {
				assert.Equal(t, requested.GitlabIDs, allowed.GitlabIDs)
				assert.Contains(t, allowed.GitlabUsernames, "someone-gl")
				assert.Empty(t, skipped)
			} else {
				assert.Empty(t, allowed.GitlabIDs)
				assert.Empty(t, allowed.GitlabUsernames)
				assert.Equal(t, skippedKeys, skipped)
			}
			assert.Equal(t, 2, inner.callCount("someone"))
			assert.Same(t, cache, svc.auth0Identities, "verification must not replace the shared history cache")

			// A denied signing check must not disable history's rate-limit fallback.
			if errors.Is(tc.live.err, errAuth0RateLimited) {
				history, err = svc.GetMyClas(ctx, caller, requested, nil, nil)
				require.NoError(t, err)
				assert.Empty(t, history.SkippedIdentities)
			}
		})
	}
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
	inner.started = make(chan context.Context, 1)
	release := sync.OnceFunc(func() { close(inner.block) })
	defer release()
	cache, _ := newTestAuth0Cache(inner)

	testCtx, cancelTest := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelTest()
	leaderCtx, cancelLeader := context.WithCancel(testCtx)
	defer cancelLeader()
	leaderErr := make(chan error, 1)
	go func() {
		_, err := cache.UserIdentities(leaderCtx, "someone")
		leaderErr <- err
	}()

	var lookupCtx context.Context
	select {
	case lookupCtx = <-inner.started:
	case <-testCtx.Done():
		t.Fatal("inner lookup did not start")
	}

	waiterCtx := &auth0WaiterContext{Context: testCtx, waiting: make(chan struct{})}
	waiterResult := make(chan auth0Result, 1)
	go func() {
		identities, err := cache.UserIdentities(waiterCtx, "someone")
		waiterResult <- auth0Result{identities: identities, err: err}
	}()
	select {
	case <-waiterCtx.waiting:
	case <-testCtx.Done():
		t.Fatal("follower did not join the shared lookup")
	}

	cancelLeader()
	select {
	case err := <-leaderErr:
		require.ErrorIs(t, err, context.Canceled)
	case <-testCtx.Done():
		t.Fatal("cancelled leader did not return")
	}
	require.NoError(t, lookupCtx.Err(), "leader cancellation must not cancel the shared lookup")

	release()
	select {
	case result := <-waiterResult:
		require.NoError(t, result.err)
		assert.Equal(t, gitlabIdentity, result.identities)
	case <-testCtx.Done():
		t.Fatal("follower did not receive the shared result")
	}

	// the detached lookup filled the cache, rather than starting a second lookup
	identities, err := cache.UserIdentities(testCtx, "someone")
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

// TestSensitiveMyClaOperationsBypassAuth0Cache keeps history's cache separate from
// PDF/manager disclosure and manager-request side effects, for fresh and stale evidence.
func TestSensitiveMyClaOperationsBypassAuth0Cache(t *testing.T) {
	const (
		pdfOperation            = "pdf"
		managerRequestOperation = "manager-request"
	)
	for _, operation := range []string{pdfOperation, "managers", managerRequestOperation} {
		t.Run(operation, func(t *testing.T) {
			for _, tc := range []struct {
				name       string
				age        time.Duration
				initial    []Auth0Identity
				live       auth0Result
				wantLinked bool
			}{
				{name: "fresh link removed", age: time.Minute, initial: gitlabIdentity},
				{name: "stale link removed", age: 30 * time.Minute, initial: gitlabIdentity},
				{name: "fresh link rate limited", age: time.Minute, initial: gitlabIdentity, live: auth0Result{err: rateLimited}},
				{name: "stale link rate limited", age: 30 * time.Minute, initial: gitlabIdentity, live: auth0Result{err: rateLimited}},
				{name: "new link bypasses cached empty result", age: time.Minute, live: auth0Result{identities: gitlabIdentity}, wantLinked: true},
				{name: "current link verified", age: time.Minute, initial: gitlabIdentity, live: auth0Result{identities: gitlabIdentity}, wantLinked: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					inner := newFakeAuth0Inner()
					inner.program(someoneLF, auth0Result{identities: tc.initial}, tc.live)
					cache, clock := newTestAuth0Cache(inner)
					repo, signaturesService, companies := managersFixture()
					// Only Auth0, never an LF-linked EasyCLA record, proves this historical account.
					historical := repo.byLFUsername[someoneLF][0]
					historical.LfUsername = ""
					historical.GitlabID = "777"
					historical.GitlabUsername = "someone-gl"
					repo.byLFUsername = nil
					repo.byGitlabID = map[int64][]*v1Models.User{777: {historical}}
					repo.byGitlabUsername = map[string][]*v1Models.User{"someone-gl": {historical}}
					svc, eventsService, sent := newRequestTestService(repo, signaturesService, companies)
					svc.auth0Identities = cache
					documentChecks, presigns := 0, 0
					svc.documentExists = func(string) (bool, error) {
						documentChecks++
						return true, nil
					}
					svc.presign = func(string) (string, error) {
						presigns++
						return "https://example.org/signed.pdf", nil
					}
					caller := &Caller{Username: someoneLF}
					requested := &Identity{GitlabIDs: []int64{777}, GitlabUsernames: []string{"someone-gl"}}
					ctx := context.Background()

					history, err := svc.GetMyClas(ctx, caller, requested, nil, nil)
					require.NoError(t, err)
					assert.Equal(t, len(tc.initial) == 0, len(history.SkippedIdentities) > 0)
					require.Equal(t, 1, inner.callCount(someoneLF))
					clock.Advance(tc.age)
					managerLookups := signaturesService.cclaCalls

					var result interface{}
					switch operation {
					case pdfOperation:
						result, err = svc.GetMyClaPdfURL(ctx, caller, requested, "sig-icla")
					case "managers":
						result, err = svc.GetMyClaManagers(ctx, caller, requested, "sig-ecla", nil, nil)
					case managerRequestOperation:
						result, err = svc.CreateMyClaManagerRequest(ctx, caller, requested, "sig-ecla",
							requestInput("contact", []string{"manager-one"}, "please contact me"))
					}
					require.NoError(t, err)
					assert.Equal(t, 2, inner.callCount(someoneLF), "each operation must verify live ownership")
					assert.Same(t, cache, svc.auth0Identities, "the shared history cache must remain intact")
					if tc.wantLinked {
						require.NotNil(t, result)
					} else {
						assert.Nil(t, result)
						assert.Equal(t, managerLookups, signaturesService.cclaCalls, "denial must not load manager details")
					}
					wantPDF := 0
					if tc.wantLinked && operation == pdfOperation {
						wantPDF = 1
					}
					assert.Equal(t, wantPDF, documentChecks)
					assert.Equal(t, wantPDF, presigns)
					wantWrites := 0
					if tc.wantLinked && operation == managerRequestOperation {
						wantWrites = 1
					}
					assert.Len(t, *sent, wantWrites)
					assert.Len(t, eventsService.logged, wantWrites)

					if errors.Is(tc.live.err, errAuth0RateLimited) {
						history, err = svc.GetMyClas(ctx, caller, requested, nil, nil)
						require.NoError(t, err)
						assert.Empty(t, history.SkippedIdentities, "history must retain rate-limit fallback")
						assert.Len(t, history.Clas, 2)
					}
				})
			}
		})
	}
}
