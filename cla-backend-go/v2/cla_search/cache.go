// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_search

import (
	"context"
	"os"
	"sync"
	"time"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
)

// DefaultCacheTTL is how long a scanned table is served from memory before the next search re-scans
// it. It is deliberately longer than a typical Lambda execution environment lives, so a container
// normally scans each table once and never again - the reference data changes on the timescale of
// project onboarding, not of searches.
const DefaultCacheTTL = 30 * time.Minute

// cacheTTLEnvVar overrides DefaultCacheTTL with any duration Go can parse, "0" disabling the cache
const cacheTTLEnvVar = "CLA_SEARCH_CACHE_TTL"

func cacheTTL() time.Duration {
	value := os.Getenv(cacheTTLEnvVar)
	if value == "" {
		return DefaultCacheTTL
	}
	ttl, err := time.ParseDuration(value)
	if err != nil || ttl < 0 {
		log.WithField(cacheTTLEnvVar, value).Warn("unable to parse the CLA Group search cache TTL - using the default")
		return DefaultCacheTTL
	}
	return ttl
}

// tableCache holds the rows of one scanned table until they age out. The rows are shared with every
// search that reads them and must be treated as read-only.
type tableCache[T any] struct {
	name   string
	ttl    time.Duration
	load   func(context.Context) ([]T, error)
	flight singleflight.Group

	mu       sync.RWMutex
	rows     []T
	loadedAt time.Time
}

func newTableCache[T any](name string, ttl time.Duration, load func(context.Context) ([]T, error)) *tableCache[T] {
	return &tableCache[T]{name: name, ttl: ttl, load: load}
}

// get returns the cached rows, scanning the table when they are absent or stale. Concurrent misses
// share a single scan.
func (c *tableCache[T]) get(ctx context.Context) ([]T, error) {
	if rows, ok := c.fresh(); ok {
		return rows, nil
	}

	f := logrus.Fields{"functionName": "v2.cla_search.cache.get", "tableName": c.name}
	_, err, shared := c.flight.Do(c.name, func() (interface{}, error) {
		// a scan that finished while this call waited for the lock makes this one unnecessary
		if _, ok := c.fresh(); ok {
			return nil, nil
		}
		rows, loadErr := c.load(ctx)
		if loadErr != nil {
			return nil, loadErr
		}
		c.mu.Lock()
		c.rows, c.loadedAt = rows, time.Now()
		c.mu.Unlock()
		log.WithFields(f).Debugf("cache miss - loaded %d rows", len(rows))
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	if shared {
		log.WithFields(f).Debug("cache miss - served by a concurrent load")
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rows, nil
}

func (c *tableCache[T]) fresh() ([]T, bool) {
	c.mu.RLock()
	rows, loadedAt := c.rows, c.loadedAt
	c.mu.RUnlock()
	if loadedAt.IsZero() || time.Since(loadedAt) >= c.ttl {
		return nil, false
	}
	return rows, true
}

// keyedCache holds one tableCache per lookup key, for a read whose cost justifies caching but whose
// result set depends on the key
type keyedCache[T any] struct {
	ttl  time.Duration
	load func(context.Context, string) ([]T, error)

	mu      sync.Mutex
	entries map[string]*tableCache[T]
}

func newKeyedCache[T any](ttl time.Duration, load func(context.Context, string) ([]T, error)) *keyedCache[T] {
	return &keyedCache[T]{ttl: ttl, load: load, entries: map[string]*tableCache[T]{}}
}

func (k *keyedCache[T]) get(ctx context.Context, key string) ([]T, error) {
	k.mu.Lock()
	entry, ok := k.entries[key]
	if !ok {
		entry = newTableCache(key, k.ttl, func(loadCtx context.Context) ([]T, error) { return k.load(loadCtx, key) })
		k.entries[key] = entry
	}
	k.mu.Unlock()
	return entry.get(ctx)
}

// cachedRepository serves the scanned tables from memory and passes the indexed repository lookups,
// whose keys have no useful cache locality, straight through
type cachedRepository struct {
	Repository
	claGroups *tableCache[*ClaGroupRow]
	mappings  *tableCache[*ProjectMappingRow]
	github    *tableCache[*OrgRow]
	gitlab    *tableCache[*OrgRow]
	gerrit    *tableCache[*OrgRow]

	// listing every repository of an organization is the one indexed read expensive enough to cache -
	// a large organization runs to hundreds of rows, and a pasted URL under it repeats the same listing
	orgRepositories *keyedCache[*RepositoryRow]
}

func newCachedRepository(repo Repository, ttl time.Duration) Repository {
	if ttl == 0 {
		return repo
	}
	return &cachedRepository{
		Repository: repo,
		orgRepositories: newKeyedCache(ttl, func(ctx context.Context, organizationName string) ([]*RepositoryRow, error) {
			return repo.GetRepositoriesByOrganization(ctx, []string{organizationName})
		}),
		claGroups: newTableCache("projects", ttl, repo.GetClaGroups),
		mappings:  newTableCache("projects-cla-groups", ttl, repo.GetProjectMappings),
		github:    newTableCache("github-orgs", ttl, repo.GetGithubOrgs),
		gitlab:    newTableCache("gitlab-orgs", ttl, repo.GetGitlabOrgs),
		gerrit:    newTableCache("gerrit-instances", ttl, repo.GetGerritInstances),
	}
}

func (c *cachedRepository) GetClaGroups(ctx context.Context) ([]*ClaGroupRow, error) {
	return c.claGroups.get(ctx)
}

func (c *cachedRepository) GetProjectMappings(ctx context.Context) ([]*ProjectMappingRow, error) {
	return c.mappings.get(ctx)
}

func (c *cachedRepository) GetGithubOrgs(ctx context.Context) ([]*OrgRow, error) {
	return c.github.get(ctx)
}

func (c *cachedRepository) GetGitlabOrgs(ctx context.Context) ([]*OrgRow, error) {
	return c.gitlab.get(ctx)
}

func (c *cachedRepository) GetGerritInstances(ctx context.Context) ([]*OrgRow, error) {
	return c.gerrit.get(ctx)
}

func (c *cachedRepository) GetRepositoriesByOrganization(ctx context.Context, organizationNames []string) ([]*RepositoryRow, error) {
	var rows []*RepositoryRow
	for _, organizationName := range organizationNames {
		cached, err := c.orgRepositories.get(ctx, organizationName)
		if err != nil {
			return nil, err
		}
		rows = append(rows, cached...)
	}
	return rows, nil
}
