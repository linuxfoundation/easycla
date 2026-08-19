// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_search

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRepo struct {
	claGroups []*ClaGroupRow
	mappings  []*ProjectMappingRow
	github    []*OrgRow
	gitlab    []*OrgRow
	gerrit    []*OrgRow
	repos     map[string][]*RepositoryRow
	orgRepos  map[string][]*RepositoryRow

	failOn string

	mu           sync.Mutex
	repoQueries  [][]string
	orgQueries   [][]string
	loaderCalls  int
	repoCalls    int
	callSequence []string
}

func (f *fakeRepo) note(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callSequence = append(f.callSequence, name)
	f.loaderCalls++
	if f.failOn == name {
		return errors.New("boom: " + name)
	}
	return nil
}

func (f *fakeRepo) GetClaGroups(_ context.Context) ([]*ClaGroupRow, error) {
	if err := f.note("claGroups"); err != nil {
		return nil, err
	}
	return f.claGroups, nil
}

func (f *fakeRepo) GetProjectMappings(_ context.Context) ([]*ProjectMappingRow, error) {
	if err := f.note("mappings"); err != nil {
		return nil, err
	}
	return f.mappings, nil
}

func (f *fakeRepo) GetGithubOrgs(_ context.Context) ([]*OrgRow, error) {
	if err := f.note("github"); err != nil {
		return nil, err
	}
	return f.github, nil
}

func (f *fakeRepo) GetGitlabOrgs(_ context.Context) ([]*OrgRow, error) {
	if err := f.note("gitlab"); err != nil {
		return nil, err
	}
	return f.gitlab, nil
}

func (f *fakeRepo) GetGerritInstances(_ context.Context) ([]*OrgRow, error) {
	if err := f.note("gerrit"); err != nil {
		return nil, err
	}
	return f.gerrit, nil
}

func (f *fakeRepo) GetRepositoriesByName(_ context.Context, names []string) ([]*RepositoryRow, error) {
	f.mu.Lock()
	f.repoQueries = append(f.repoQueries, names)
	f.repoCalls++
	f.mu.Unlock()
	if f.failOn == "repositories" {
		return nil, errors.New("boom: repositories")
	}
	var rows []*RepositoryRow
	for _, name := range names {
		rows = append(rows, f.repos[name]...)
	}
	return rows, nil
}

func (f *fakeRepo) GetRepositoriesByOrganization(_ context.Context, organizationNames []string) ([]*RepositoryRow, error) {
	f.mu.Lock()
	f.orgQueries = append(f.orgQueries, organizationNames)
	f.mu.Unlock()
	if f.failOn == "organizationRepositories" {
		return nil, errors.New("boom: organizationRepositories")
	}
	var rows []*RepositoryRow
	for _, name := range organizationNames {
		rows = append(rows, f.orgRepos[name]...)
	}
	return rows, nil
}

// sampleRepo mirrors the production shape: two CLA groups behind GitHub orgs, one foundation-level
// CLA group behind a Gerrit instance, and one behind a GitLab group
func sampleRepo() *fakeRepo {
	return &fakeRepo{
		claGroups: []*ClaGroupRow{
			{ClaGroupID: "cg-kube", Name: "Kubernetes CLA", ExternalID: "a09-kube", IclaEnabled: true, CclaEnabled: true},
			{ClaGroupID: "cg-otio", Name: "OpenTimelineIO CLA", ExternalID: "a09-otio", CclaEnabled: true},
			{ClaGroupID: "cg-onap", Name: "ONAP CLA", ExternalID: "a09-onap-f", IclaEnabled: true},
			{ClaGroupID: "cg-orphan", Name: "Kubernetes Edge CLA"},
		},
		mappings: []*ProjectMappingRow{
			{ClaGroupID: "cg-kube", ClaGroupName: "Kubernetes CLA", ProjectSFID: "sfid-kube", ProjectName: "Kubernetes", FoundationSFID: "sfid-cncf", FoundationName: "CNCF"},
			{ClaGroupID: "cg-otio", ClaGroupName: "OpenTimelineIO CLA", ProjectSFID: "sfid-otio", ProjectName: "OpenTimelineIO", FoundationSFID: "sfid-aswf", FoundationName: "Academy Software Foundation"},
			{ClaGroupID: "cg-onap", ClaGroupName: "ONAP CLA", ProjectSFID: "sfid-onap-f", ProjectName: "ONAP Foundation Level", FoundationSFID: "sfid-onap-f", FoundationName: "ONAP"},
			{ClaGroupID: "cg-multi", ClaGroupName: "Shared CLA", ProjectSFID: "sfid-a", ProjectName: "Shared Project A", FoundationSFID: "sfid-root"},
			{ClaGroupID: "cg-multi", ClaGroupName: "Shared CLA", ProjectSFID: "sfid-b", ProjectName: "Shared Project B", FoundationSFID: "sfid-root"},
		},
		github: []*OrgRow{
			{Name: "kubernetes", Source: sourceGitHub, ProjectSFID: "sfid-kube"},
			{Name: "kubernetes-sigs", Source: sourceGitHub, ProjectSFID: "sfid-kube"},
			{Name: "OpenTimelineIO", Source: sourceGitHub, ProjectSFID: "sfid-otio"},
		},
		gitlab: []*OrgRow{
			{Name: "onap", URL: "https://gitlab.com/groups/onap", Source: sourceGitLab, ProjectSFID: "sfid-onap-f"},
		},
		gerrit: []*OrgRow{
			{Name: "ONAP", URL: "https://gerrit.onap.org", Source: sourceGerrit, ClaGroupID: "cg-onap"},
		},
		repos: map[string][]*RepositoryRow{
			"OpenTimelineIO/OpenTimelineIO-Java-Bindings": {{Name: "OpenTimelineIO/OpenTimelineIO-Java-Bindings", URL: "https://github.com/OpenTimelineIO/OpenTimelineIO-Java-Bindings", Type: sourceGitHub, ClaGroupID: "cg-otio"}},
			"onap/oom/oom": {{Name: "onap/oom/oom", URL: "https://gitlab.com/onap/oom/oom", Type: sourceGitLab, ClaGroupID: "cg-onap"}},
		},
		orgRepos: map[string][]*RepositoryRow{
			"OpenTimelineIO": {
				{Name: "OpenTimelineIO/OpenTimelineIO-Java-Bindings", URL: "https://github.com/OpenTimelineIO/OpenTimelineIO-Java-Bindings", Type: sourceGitHub, ClaGroupID: "cg-otio"},
				{Name: "OpenTimelineIO/otio-plugin-template", URL: "https://github.com/OpenTimelineIO/otio-plugin-template", Type: sourceGitHub, ClaGroupID: "cg-otio"},
			},
		},
	}
}

func resultByID(list *models.ClaSearchList, claGroupID string) *models.ClaSearchResult {
	for i := range list.Results {
		if list.Results[i].ClaGroupID == claGroupID {
			return &list.Results[i]
		}
	}
	return nil
}

func ids(list *models.ClaSearchList) []string {
	out := make([]string, 0, len(list.Results))
	for i := range list.Results {
		out = append(out, list.Results[i].ClaGroupID)
	}
	return out
}

func TestSearchByClaGroupName(t *testing.T) {
	svc := NewService(sampleRepo())
	list, err := svc.Search(context.Background(), "kubernetes cla", 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"cg-kube"}, ids(list))
	assert.Equal(t, int64(1), list.ResultCount)
	assert.False(t, list.Truncated)

	kube := resultByID(list, "cg-kube")
	require.NotNil(t, kube)
	assert.Equal(t, []string{matchClaGroup}, kube.MatchTypes)
	assert.Equal(t, "Kubernetes CLA", kube.ClaGroupName)
	assert.Equal(t, "Kubernetes", kube.ProjectName)
	assert.Equal(t, "sfid-kube", kube.ProjectSFID)
	assert.Equal(t, "sfid-cncf", kube.FoundationSFID)
	assert.Equal(t, "a09-kube", kube.ProjectExternalID)
	assert.True(t, kube.IclaEnabled)
	assert.True(t, kube.CclaEnabled)
}

func TestSearchByProjectName(t *testing.T) {
	svc := NewService(sampleRepo())
	list, err := svc.Search(context.Background(), "opentimelineio", 0)
	require.NoError(t, err)
	require.Equal(t, []string{"cg-otio"}, ids(list))
	assert.Equal(t, []string{matchClaGroup, matchOrganization, matchProject}, list.Results[0].MatchTypes)
}

func TestSearchByFoundationName(t *testing.T) {
	svc := NewService(sampleRepo())
	list, err := svc.Search(context.Background(), "academy software", 0)
	require.NoError(t, err)
	require.Equal(t, []string{"cg-otio"}, ids(list))
	assert.Equal(t, []string{matchProject}, list.Results[0].MatchTypes)
}

func TestSearchByOrgNameCarriesProvenance(t *testing.T) {
	svc := NewService(sampleRepo())
	list, err := svc.Search(context.Background(), "kubernetes-sigs", 0)
	require.NoError(t, err)
	require.Equal(t, []string{"cg-kube"}, ids(list))
	assert.Equal(t, []string{matchOrganization}, list.Results[0].MatchTypes)
	// every organization linked to the CLA Group is returned, and the GitHub URL is derived
	assert.Equal(t, []models.ClaSearchOrg{
		{Name: "kubernetes", Source: sourceGitHub, URL: "https://github.com/kubernetes"},
		{Name: "kubernetes-sigs", Source: sourceGitHub, URL: "https://github.com/kubernetes-sigs"},
	}, list.Results[0].Organizations)
}

func TestSearchGitlabAndGerritProvenanceAreReturned(t *testing.T) {
	svc := NewService(sampleRepo())
	list, err := svc.Search(context.Background(), "onap", 0)
	require.NoError(t, err)
	require.Equal(t, []string{"cg-onap"}, ids(list))
	onap := list.Results[0]
	assert.Equal(t, []string{matchClaGroup, matchOrganization, matchProject}, onap.MatchTypes)
	assert.Equal(t, []models.ClaSearchOrg{
		{Name: "ONAP", Source: sourceGerrit, URL: "https://gerrit.onap.org"},
		{Name: "onap", Source: sourceGitLab, URL: "https://gitlab.com/groups/onap"},
	}, onap.Organizations)
	// foundation-level CLA group resolves to its foundation
	assert.Equal(t, "ONAP", onap.ProjectName)
	assert.Equal(t, "sfid-onap-f", onap.ProjectSFID)
}

func TestSearchResolvesPastedRepoURL(t *testing.T) {
	for _, term := range []string{
		"https://github.com/OpenTimelineIO/OpenTimelineIO-Java-Bindings",
		"https://github.com/OpenTimelineIO/OpenTimelineIO-Java-Bindings.git",
		"https://github.com/OpenTimelineIO/OpenTimelineIO-Java-Bindings/",
		"OpenTimelineIO/OpenTimelineIO-Java-Bindings",
	} {
		t.Run(term, func(t *testing.T) {
			svc := NewService(sampleRepo())
			list, err := svc.Search(context.Background(), term, 0)
			require.NoError(t, err)
			require.Equal(t, []string{"cg-otio"}, ids(list))
			assert.Contains(t, list.Results[0].MatchTypes, matchRepository)
			assert.Equal(t, "OpenTimelineIO/OpenTimelineIO-Java-Bindings", list.Results[0].MatchedRepositoryName)
			assert.Equal(t, "https://github.com/OpenTimelineIO/OpenTimelineIO-Java-Bindings", list.Results[0].MatchedRepositoryURL)
		})
	}
}

func TestSearchResolvesNestedGitlabRepoURL(t *testing.T) {
	svc := NewService(sampleRepo())
	list, err := svc.Search(context.Background(), "https://gitlab.com/onap/oom/oom", 0)
	require.NoError(t, err)
	require.Equal(t, []string{"cg-onap"}, ids(list))
	assert.Contains(t, list.Results[0].MatchTypes, matchRepository)
}

func TestSearchResolvesGerritHostURL(t *testing.T) {
	svc := NewService(sampleRepo())
	list, err := svc.Search(context.Background(), "https://gerrit.onap.org/r/aai/aai-common", 0)
	require.NoError(t, err)
	require.Equal(t, []string{"cg-onap"}, ids(list))
	assert.Equal(t, []string{matchOrganization}, list.Results[0].MatchTypes)
}

func TestSearchDoesNotMatchEveryGroupOnForgeHost(t *testing.T) {
	svc := NewService(sampleRepo())
	list, err := svc.Search(context.Background(), "https://github.com/unknown-org/unknown-repo", 0)
	require.NoError(t, err)
	assert.Empty(t, list.Results)
	assert.Equal(t, int64(0), list.ResultCount)
	assert.NotNil(t, list.Results)
}

func TestSearchResolvesLowerCasedPasteOfMixedCaseRepoURL(t *testing.T) {
	repo := sampleRepo()
	svc := NewService(repo)
	list, err := svc.Search(context.Background(), "https://github.com/opentimelineio/opentimelineio-java-bindings", 0)
	require.NoError(t, err)
	require.Equal(t, []string{"cg-otio"}, ids(list))
	assert.Contains(t, list.Results[0].MatchTypes, matchRepository)
	assert.Equal(t, "OpenTimelineIO/OpenTimelineIO-Java-Bindings", list.Results[0].MatchedRepositoryName)
	assert.Equal(t, [][]string{{"OpenTimelineIO"}}, repo.orgQueries)
}

func TestSearchOwnerOfOrganizationMatchesWhenRepositoryHasNoRecord(t *testing.T) {
	repo := sampleRepo()
	svc := NewService(repo)
	list, err := svc.Search(context.Background(), "https://github.com/kubernetes/a-brand-new-repo", 0)
	require.NoError(t, err)
	require.Equal(t, []string{"cg-kube"}, ids(list))
	assert.Equal(t, []string{matchOrganization}, list.Results[0].MatchTypes)
	assert.Empty(t, list.Results[0].MatchedRepositoryName)
}

func TestSearchForgeNameDoesNotMatchEveryOrganizationOnIt(t *testing.T) {
	for _, term := range []string{"gitlab", "github", "gitlab.com"} {
		t.Run(term, func(t *testing.T) {
			list, err := NewService(sampleRepo()).Search(context.Background(), term, 0)
			require.NoError(t, err)
			assert.Empty(t, list.Results)
		})
	}
}

func TestSearchMatchesSelfHostedInstanceHost(t *testing.T) {
	list, err := NewService(sampleRepo()).Search(context.Background(), "gerrit.onap.org", 0)
	require.NoError(t, err)
	require.Equal(t, []string{"cg-onap"}, ids(list))
}

func TestSearchQueriesRepositoryNameCasePreservedAndLowered(t *testing.T) {
	repo := sampleRepo()
	svc := NewService(repo)
	_, err := svc.Search(context.Background(), "https://github.com/OpenTimelineIO/OpenTimelineIO-Java-Bindings", 0)
	require.NoError(t, err)
	require.Equal(t, [][]string{{
		"OpenTimelineIO/OpenTimelineIO-Java-Bindings",
		"opentimelineio/opentimelineio-java-bindings",
	}}, repo.repoQueries)
}

func TestSearchSkipsRepositoryLookupWithoutAPath(t *testing.T) {
	repo := sampleRepo()
	svc := NewService(repo)
	_, err := svc.Search(context.Background(), "kubernetes", 0)
	require.NoError(t, err)
	assert.Zero(t, repo.repoCalls)
}

func TestSearchAmbiguousMultiProjectClaGroupOmitsProjectFields(t *testing.T) {
	svc := NewService(sampleRepo())
	list, err := svc.Search(context.Background(), "shared", 0)
	require.NoError(t, err)
	require.Equal(t, []string{"cg-multi"}, ids(list))
	assert.Empty(t, list.Results[0].ProjectName)
	assert.Empty(t, list.Results[0].ProjectSFID)
	assert.Equal(t, "sfid-root", list.Results[0].FoundationSFID)
	// the CLA group record is absent from the projects table, the mapping supplies the name
	assert.Equal(t, "Shared CLA", list.Results[0].ClaGroupName)
	assert.Equal(t, []models.ClaSearchOrg{}, list.Results[0].Organizations)
}

func TestSearchIsCaseInsensitiveAndTrimmed(t *testing.T) {
	svc := NewService(sampleRepo())
	list, err := svc.Search(context.Background(), "  KUBERNETES-SIGS  ", 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"cg-kube"}, ids(list))
	assert.Equal(t, "  KUBERNETES-SIGS  ", list.SearchTerm)
}

func TestSearchRanksExactBeforePrefixBeforeSubstring(t *testing.T) {
	repo := &fakeRepo{claGroups: []*ClaGroupRow{
		{ClaGroupID: "cg-sub", Name: "The Zeta Project"},
		{ClaGroupID: "cg-exact", Name: "zeta"},
		{ClaGroupID: "cg-prefix", Name: "Zeta Networking"},
	}}
	list, err := NewService(repo).Search(context.Background(), "zeta", 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"cg-exact", "cg-prefix", "cg-sub"}, ids(list))
}

func TestSearchTruncatesAtLimit(t *testing.T) {
	repo := &fakeRepo{}
	for i := 0; i < 5; i++ {
		repo.claGroups = append(repo.claGroups, &ClaGroupRow{ClaGroupID: fmt.Sprintf("cg-%d", i), Name: fmt.Sprintf("Zeta %d CLA", i)})
	}
	list, err := NewService(repo).Search(context.Background(), "zeta", 3)
	require.NoError(t, err)
	assert.True(t, list.Truncated)
	assert.Equal(t, int64(3), list.ResultCount)
	assert.Equal(t, []string{"cg-0", "cg-1", "cg-2"}, ids(list))
}

func TestSearchNotTruncatedAtExactlyLimit(t *testing.T) {
	repo := &fakeRepo{claGroups: []*ClaGroupRow{
		{ClaGroupID: "cg-0", Name: "Zeta A CLA"},
		{ClaGroupID: "cg-1", Name: "Zeta B CLA"},
	}}
	list, err := NewService(repo).Search(context.Background(), "zeta", 2)
	require.NoError(t, err)
	assert.False(t, list.Truncated)
	assert.Equal(t, int64(2), list.ResultCount)
}

func TestSearchDefaultsLimit(t *testing.T) {
	repo := &fakeRepo{}
	for i := 0; i < DefaultLimit+1; i++ {
		repo.claGroups = append(repo.claGroups, &ClaGroupRow{ClaGroupID: fmt.Sprintf("cg-%02d", i), Name: fmt.Sprintf("Zeta %02d CLA", i)})
	}
	list, err := NewService(repo).Search(context.Background(), "zeta", 0)
	require.NoError(t, err)
	assert.True(t, list.Truncated)
	assert.Equal(t, int64(DefaultLimit), list.ResultCount)
}

func TestSearchNoMatchReturnsEmptyList(t *testing.T) {
	list, err := NewService(sampleRepo()).Search(context.Background(), "nothing-matches-this", 0)
	require.NoError(t, err)
	assert.Equal(t, int64(0), list.ResultCount)
	assert.False(t, list.Truncated)
	assert.NotNil(t, list.Results)
}

func TestSearchPropagatesSourceErrors(t *testing.T) {
	for _, source := range []string{"claGroups", "mappings", "github", "gitlab", "gerrit", "repositories"} {
		t.Run(source, func(t *testing.T) {
			repo := sampleRepo()
			repo.failOn = source
			list, err := NewService(repo).Search(context.Background(), "OpenTimelineIO/OpenTimelineIO-Java-Bindings", 0)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "boom: "+source)
			assert.Nil(t, list)
		})
	}
}

func TestSearchRunsSourcesConcurrently(t *testing.T) {
	repo := sampleRepo()
	_, err := NewService(repo).Search(context.Background(), "onap/oom/oom", 0)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"claGroups", "mappings", "github", "gitlab", "gerrit"}, repo.callSequence)
	assert.Equal(t, 1, repo.repoCalls)
}

func TestRepositoryPath(t *testing.T) {
	for _, tc := range []struct {
		term     string
		expected string
		variants []string
	}{
		{"kubernetes", "", nil},
		{"has space/repo", "", nil},
		{"not-a-url://", "", nil},
		{"Owner/Repo", "Owner/Repo", []string{"Owner/Repo", "owner/repo"}},
		{"owner/repo", "owner/repo", []string{"owner/repo"}},
		{"https://gitlab.com/groups/onap", "", nil},
		{"https://gitlab.com/onap/oom/oom", "onap/oom/oom", []string{"onap/oom/oom"}},
		{"https://github.com/Owner/Repo.git", "Owner/Repo", []string{"Owner/Repo", "owner/repo"}},
	} {
		path := repositoryPath(tc.term)
		assert.Equal(t, tc.expected, path, tc.term)
		if path != "" {
			assert.Equal(t, tc.variants, nameVariants(path), tc.term)
		}
	}
}
