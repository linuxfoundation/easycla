package githublegacy

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// This file ports the in-memory GitHub cache from the legacy Python implementation:
// - cla/models/github_models.py (TTLCache + github_user_cache)
// - cla/models/github_models.update_cache_after_signature()
//
// The cache is best-effort and only impacts warm Lambda containers.

const (
	// NegativeCacheTTL mirrors Python NEGATIVE_CACHE_TTL (3 minutes).
	NegativeCacheTTL = 180 * time.Second
	// ProjectCacheTTL mirrors Python PROJECT_CACHE_TTL (3 hours).
	ProjectCacheTTL = 10800 * time.Second
	// DefaultCacheTTL mirrors Python github_user_cache default TTL (12 hours).
	DefaultCacheTTL = 43200 * time.Second
)

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

// TTLCache is a minimal TTL cache that mirrors the Python TTLCache behavior.
//
// Keys are strings for simplicity (the Python implementation uses tuples).
// Values are opaque and should be treated as read-only.
type TTLCache struct {
	mu   sync.Mutex
	ttl  time.Duration
	data map[string]cacheEntry
}

func NewTTLCache(defaultTTL time.Duration) *TTLCache {
	if defaultTTL <= 0 {
		defaultTTL = DefaultCacheTTL
	}
	return &TTLCache{ttl: defaultTTL, data: make(map[string]cacheEntry)}
}

func (c *TTLCache) Get(key string) (any, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(e.expiresAt) {
		delete(c.data, key)
		return nil, false
	}
	return e.value, true
}

func (c *TTLCache) Set(key string, value any) {
	if c == nil {
		return
	}
	c.SetWithTTL(key, value, c.ttl)
}

func (c *TTLCache) SetWithTTL(key string, value any, ttl time.Duration) {
	if c == nil {
		return
	}
	if ttl <= 0 {
		ttl = c.ttl
	}
	c.mu.Lock()
	c.data[key] = cacheEntry{value: value, expiresAt: time.Now().Add(ttl)}
	c.mu.Unlock()
}

func (c *TTLCache) Cleanup() {
	if c == nil {
		return
	}
	now := time.Now()
	c.mu.Lock()
	for k, e := range c.data {
		if now.After(e.expiresAt) {
			delete(c.data, k)
		}
	}
	c.mu.Unlock()
}

func (c *TTLCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	for k := range c.data {
		delete(c.data, k)
	}
	c.mu.Unlock()
}

func (c *TTLCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.data)
}

// githubUserCache mirrors the Python module-level github_user_cache.
var githubUserCache = NewTTLCache(DefaultCacheTTL)

var cleanupOnce sync.Once

func startCacheCleanup() {
	cleanupOnce.Do(func() {
		go func() {
			// Python runs cleanup hourly.
			ticker := time.NewTicker(1 * time.Hour)
			defer ticker.Stop()
			for range ticker.C {
				githubUserCache.Cleanup()
			}
		}()
	})
}

func init() {
	startCacheCleanup()
}

// ProjectCacheValue matches the per-project cached tuple:
// (user, check_aff, authorized, affiliated)
//
// We store only the user_id (string) rather than a full user object.
// The cache is best-effort and currently only updated to match legacy side-effects.
type ProjectCacheValue struct {
	UserID     string
	CheckAff   bool
	Authorized bool
	Affiliated bool
}

// CacheValue matches the general cached tuple:
// (user, check_aff)
type CacheValue struct {
	UserID   string
	CheckAff bool
}

// ClearCaches clears in-memory caches maintained by this legacy GitHub layer.
//
// Python: cla.models.github_models.clear_caches()
func ClearCaches() {
	githubUserCache.Clear()
}

// UpdateCacheAfterSignature mirrors cla.models.github_models.update_cache_after_signature().
// It marks a user as authorized for a project in the in-memory cache.
//
// NOTE: This is only used for GitHub flows in legacy Python, since only GitHub used caching.
func UpdateCacheAfterSignature(projectID, userID, githubID, githubUsername string, emails []string, affiliated bool) {
	projectID = strings.TrimSpace(projectID)
	userID = strings.TrimSpace(userID)
	githubID = strings.TrimSpace(githubID)
	githubUsername = strings.ToLower(strings.TrimSpace(githubUsername))
	if projectID == "" || userID == "" {
		return
	}
	if githubID == "" || githubUsername == "" {
		// Matches Python: skip if missing GitHub ID or username.
		return
	}
	uniqEmails := make([]string, 0, len(emails))
	seen := map[string]struct{}{}
	for _, e := range emails {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if _, ok := seen[e]; ok {
			continue
		}
		seen[e] = struct{}{}
		uniqEmails = append(uniqEmails, e)
	}
	if len(uniqEmails) == 0 {
		return
	}

	for _, email := range uniqEmails {
		projectCacheKey := fmt.Sprintf("%s|%s|%s|%s", projectID, githubID, githubUsername, email)
		cacheKey := fmt.Sprintf("%s|%s|%s", githubID, githubUsername, email)
		// Per-project cache: (user, check_aff=true, authorized=true, affiliated)
		githubUserCache.SetWithTTL(projectCacheKey, ProjectCacheValue{UserID: userID, CheckAff: true, Authorized: true, Affiliated: affiliated}, ProjectCacheTTL)
		// General cache: (user, check_aff=true)
		githubUserCache.Set(cacheKey, CacheValue{UserID: userID, CheckAff: true})
	}
}
