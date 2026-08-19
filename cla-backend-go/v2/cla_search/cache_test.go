// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_search

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func countOf(sequence []string, name string) int {
	count := 0
	for _, entry := range sequence {
		if entry == name {
			count++
		}
	}
	return count
}

func TestCacheScansEachTableOnceForRepeatedSearches(t *testing.T) {
	repo := sampleRepo()
	svc := NewService(newCachedRepository(repo, time.Minute))
	for i := 0; i < 5; i++ {
		list, err := svc.Search(context.Background(), "kubernetes cla", 0)
		require.NoError(t, err)
		require.Equal(t, []string{"cg-kube"}, ids(list))
	}
	for _, table := range []string{"claGroups", "mappings", "github", "gitlab", "gerrit"} {
		assert.Equal(t, 1, countOf(repo.callSequence, table), table)
	}
}

func TestCacheLeavesTheIndexedRepositoryLookupsUncached(t *testing.T) {
	repo := sampleRepo()
	svc := NewService(newCachedRepository(repo, time.Minute))
	for i := 0; i < 3; i++ {
		_, err := svc.Search(context.Background(), "OpenTimelineIO/OpenTimelineIO-Java-Bindings", 0)
		require.NoError(t, err)
	}
	assert.Equal(t, 3, repo.repoCalls)
}

func TestCacheReusesTheOrganizationRepositoryListing(t *testing.T) {
	repo := sampleRepo()
	svc := NewService(newCachedRepository(repo, time.Minute))
	for i := 0; i < 3; i++ {
		list, err := svc.Search(context.Background(), "https://github.com/opentimelineio/opentimelineio-java-bindings", 0)
		require.NoError(t, err)
		require.Equal(t, []string{"cg-otio"}, ids(list))
		require.Equal(t, "OpenTimelineIO/OpenTimelineIO-Java-Bindings", list.Results[0].MatchedRepositoryName)
	}
	assert.Equal(t, [][]string{{"OpenTimelineIO"}}, repo.orgQueries)
}

func TestCacheRescansWhenTheEntryIsStale(t *testing.T) {
	var loads int32
	cache := newTableCache("projects", 20*time.Millisecond, func(_ context.Context) ([]*ClaGroupRow, error) {
		atomic.AddInt32(&loads, 1)
		return []*ClaGroupRow{{ClaGroupID: "cg-1"}}, nil
	})

	rows, err := cache.get(context.Background())
	require.NoError(t, err)
	require.Len(t, rows, 1)

	_, err = cache.get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&loads))

	time.Sleep(30 * time.Millisecond)
	_, err = cache.get(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(&loads))
}

func TestCacheConcurrentMissesShareASingleScan(t *testing.T) {
	var loads int32
	cache := newTableCache("projects", time.Minute, func(_ context.Context) ([]*ClaGroupRow, error) {
		atomic.AddInt32(&loads, 1)
		time.Sleep(50 * time.Millisecond)
		return []*ClaGroupRow{{ClaGroupID: "cg-1"}}, nil
	})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rows, err := cache.get(context.Background())
			assert.NoError(t, err)
			assert.Len(t, rows, 1)
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), atomic.LoadInt32(&loads))
}

func TestCacheFillSurvivesTheCancellationOfTheRequestThatTriggeredIt(t *testing.T) {
	var loads int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	cache := newTableCache("projects", time.Minute, func(ctx context.Context) ([]*ClaGroupRow, error) {
		atomic.AddInt32(&loads, 1)
		started <- struct{}{}
		<-release
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return []*ClaGroupRow{{ClaGroupID: "cg-1"}}, nil
	})

	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderErr := make(chan error, 1)
	go func() {
		_, err := cache.get(leaderCtx)
		leaderErr <- err
	}()
	<-started

	type waiterResult struct {
		rows []*ClaGroupRow
		err  error
	}
	waiter := make(chan waiterResult, 1)
	go func() {
		rows, err := cache.get(context.Background())
		waiter <- waiterResult{rows: rows, err: err}
	}()
	time.Sleep(50 * time.Millisecond)

	cancelLeader()
	assert.ErrorIs(t, <-leaderErr, context.Canceled)

	close(release)
	result := <-waiter
	require.NoError(t, result.err)
	require.Len(t, result.rows, 1)

	rows, err := cache.get(context.Background())
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, int32(1), atomic.LoadInt32(&loads))
}

func TestCacheDoesNotRetainAFailedScan(t *testing.T) {
	var loads int32
	cache := newTableCache("projects", time.Minute, func(_ context.Context) ([]*ClaGroupRow, error) {
		if atomic.AddInt32(&loads, 1) == 1 {
			return nil, errors.New("boom")
		}
		return []*ClaGroupRow{{ClaGroupID: "cg-1"}}, nil
	})

	_, err := cache.get(context.Background())
	require.Error(t, err)

	rows, err := cache.get(context.Background())
	require.NoError(t, err)
	assert.Len(t, rows, 1)
}

func TestCacheDisabledByAZeroTTL(t *testing.T) {
	repo := sampleRepo()
	assert.Equal(t, Repository(repo), newCachedRepository(repo, 0))

	svc := NewService(newCachedRepository(repo, 0))
	for i := 0; i < 2; i++ {
		_, err := svc.Search(context.Background(), "kubernetes", 0)
		require.NoError(t, err)
	}
	assert.Equal(t, 2, countOf(repo.callSequence, "claGroups"))
}

func TestCacheTTLFromTheEnvironment(t *testing.T) {
	for _, tc := range []struct {
		value    string
		expected time.Duration
	}{
		{"", DefaultCacheTTL},
		{"90s", 90 * time.Second},
		{"0", 0},
		{"-1m", DefaultCacheTTL},
		{"not-a-duration", DefaultCacheTTL},
	} {
		t.Run(tc.value, func(t *testing.T) {
			t.Setenv(cacheTTLEnvVar, tc.value)
			assert.Equal(t, tc.expected, cacheTTL())
		})
	}
}
