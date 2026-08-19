// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_search

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"golang.org/x/sync/errgroup"
)

const (
	sourceGitHub = "github"
	sourceGitLab = "gitlab"
	sourceGerrit = "gerrit"

	matchClaGroup     = "claGroup"
	matchProject      = "project"
	matchOrganization = "organization"
	matchRepository   = "repository"

	// DefaultLimit is the result cap applied when the caller provides no limit
	DefaultLimit = 20

	// MinSearchTermLength is the shortest search term accepted
	MinSearchTermLength = 3
)

// match quality, lowest sorts first
const (
	rankExact     = 0
	rankPrefix    = 1
	rankSubstring = 2
)

// forgeHosts are the shared repository hosts whose hostname carries no CLA Group signal, so a
// pasted URL on one of them is matched by its path only, and the forge each one identifies
var forgeHosts = map[string]string{
	"github.com":     sourceGitHub,
	"www.github.com": sourceGitHub,
	"gitlab.com":     sourceGitLab,
	"www.gitlab.com": sourceGitLab,
}

// Service interface defines the CLA Group search service
type Service interface {
	Search(ctx context.Context, searchTerm string, limit int64) (*models.ClaSearchList, error)
}

type service struct {
	repo Repository
}

// NewService creates a new instance of the CLA Group search service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// sources holds the reference data the four search sources are matched against
type sources struct {
	claGroups []*ClaGroupRow
	mappings  []*ProjectMappingRow
	orgs      []*OrgRow
}

// Search resolves the search term against the CLA Group names, the project/foundation names, the
// linked organization names and the repository the term resolves to. Each source is searched in its
// own goroutine and the results are merged by CLA Group - the CLA Group is the signing unit.
func (s *service) Search(ctx context.Context, searchTerm string, limit int64) (*models.ClaSearchList, error) {
	rawTerm := strings.TrimSpace(searchTerm)
	term := strings.ToLower(rawTerm)
	if limit <= 0 {
		limit = DefaultLimit
	}

	// the reference data of every source and the repository the term addresses are fetched together
	var (
		src   *sources
		repos []*RepositoryRow
	)
	path, host := repositoryPath(rawTerm)
	fetch, fetchCtx := errgroup.WithContext(ctx)
	fetch.Go(func() error {
		var err error
		src, err = s.loadSources(fetchCtx)
		return err
	})
	if path != "" {
		fetch.Go(func() error {
			var err error
			repos, err = s.repo.GetRepositoriesByName(fetchCtx, nameVariants(path))
			return err
		})
	}
	if err := fetch.Wait(); err != nil {
		return nil, err
	}
	sfidToClaGroups := indexProjectSFIDs(src.mappings)

	m := newMatcher()
	searchers, searchCtx := errgroup.WithContext(ctx)
	searchers.Go(func() error { matchClaGroupNames(src.claGroups, term, m); return nil })
	searchers.Go(func() error { matchProjectNames(src.mappings, term, m); return nil })
	searchers.Go(func() error { matchOrgNames(src.orgs, term, sfidToClaGroups, m); return nil })
	searchers.Go(func() error { return s.matchRepositories(searchCtx, path, host, repos, src, sfidToClaGroups, m) })
	if err := searchers.Wait(); err != nil {
		return nil, err
	}

	return buildList(searchTerm, limit, m.matches, src, indexOrgs(src.orgs, sfidToClaGroups)), nil
}

// loadSources loads the reference data of every search source in parallel
func (s *service) loadSources(ctx context.Context) (*sources, error) {
	var (
		src    sources
		orgsMu sync.Mutex
	)
	loaders, loadCtx := errgroup.WithContext(ctx)
	loaders.Go(func() error {
		var err error
		src.claGroups, err = s.repo.GetClaGroups(loadCtx)
		return err
	})
	loaders.Go(func() error {
		var err error
		src.mappings, err = s.repo.GetProjectMappings(loadCtx)
		return err
	})
	for _, load := range []func(context.Context) ([]*OrgRow, error){s.repo.GetGithubOrgs, s.repo.GetGitlabOrgs, s.repo.GetGerritInstances} {
		loadOrgs := load
		loaders.Go(func() error {
			rows, err := loadOrgs(loadCtx)
			if err != nil {
				return err
			}
			orgsMu.Lock()
			defer orgsMu.Unlock()
			src.orgs = append(src.orgs, rows...)
			return nil
		})
	}
	if err := loaders.Wait(); err != nil {
		return nil, err
	}
	return &src, nil
}

// match is the accumulated match state of a single CLA Group
type match struct {
	types    map[string]bool
	rank     int
	repoName string
	repoURL  string
}

// matcher merges the matches the concurrent searchers produce
type matcher struct {
	mu      sync.Mutex
	matches map[string]*match
}

func newMatcher() *matcher {
	return &matcher{matches: map[string]*match{}}
}

func (m *matcher) record(claGroupID, matchType string, rank int) *match {
	if claGroupID == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	entry, ok := m.matches[claGroupID]
	if !ok {
		entry = &match{types: map[string]bool{}, rank: rank}
		m.matches[claGroupID] = entry
	}
	entry.types[matchType] = true
	if rank < entry.rank {
		entry.rank = rank
	}
	return entry
}

func (m *matcher) recordRepository(claGroupID, name, repoURL string) {
	entry := m.record(claGroupID, matchRepository, rankExact)
	if entry == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if entry.repoName == "" {
		entry.repoName, entry.repoURL = name, repoURL
	}
}

func matchClaGroupNames(claGroups []*ClaGroupRow, term string, m *matcher) {
	for _, claGroup := range claGroups {
		if rank := rankOf(claGroup.Name, term); rank >= 0 {
			m.record(claGroup.ClaGroupID, matchClaGroup, rank)
		}
	}
}

func matchProjectNames(mappings []*ProjectMappingRow, term string, m *matcher) {
	for _, mapping := range mappings {
		if rank := bestRank(term, mapping.ProjectName, mapping.FoundationName); rank >= 0 {
			m.record(mapping.ClaGroupID, matchProject, rank)
		}
	}
}

func matchOrgNames(orgs []*OrgRow, term string, sfidToClaGroups map[string][]string, m *matcher) {
	host := hostOf(term)
	for _, org := range orgs {
		rank := orgRank(org, term, host)
		if rank < 0 {
			continue
		}
		for _, claGroupID := range org.claGroupIDs(sfidToClaGroups) {
			m.record(claGroupID, matchOrganization, rank)
		}
	}
}

// orgRank matches the term against the organization name, its URL, and - for a self-hosted
// instance such as a Gerrit server - the hostname of a pasted URL under it
func orgRank(org *OrgRow, term, host string) int {
	if rank := rankOf(org.Name, term); rank >= 0 {
		return rank
	}
	if org.URL == "" {
		return -1
	}
	if strings.Contains(urlSignal(org.URL), term) {
		return rankSubstring
	}
	if host != "" && forgeHosts[host] == "" && hostOf(org.URL) == host {
		return rankSubstring
	}
	return -1
}

// matchRepositories resolves the pre-fetched repositories of a pasted repository URL or "owner/repo"
// path to the CLA Group owning that repository. A pasted URL names exactly one repository, so its
// owner organization is only consulted when no repository record answers - the case of an
// auto-enabled organization, whose repositories carry no records
func (s *service) matchRepositories(ctx context.Context, path, host string, repos []*RepositoryRow, src *sources, sfidToClaGroups map[string][]string, m *matcher) error {
	if path == "" {
		return nil
	}
	owner := path[:strings.Index(path, "/")]
	forge := forgeHosts[host]
	ownerOrgs := orgsOnHost(orgsNamed(src.orgs, owner), host, forge)

	matched := reposNamed(repos, path, host, forge)
	// the repository-name-index GSI is keyed on the case-preserved name, so a lower-cased paste of a
	// mixed-case repository misses it - the owner's repositories are then listed through the
	// organization GSI and compared case-insensitively
	if len(matched) == 0 && len(ownerOrgs) > 0 {
		listed, err := s.repo.GetRepositoriesByOrganization(ctx, organizationNames(ownerOrgs))
		if err != nil {
			return err
		}
		matched = reposNamed(listed, path, host, forge)
	}

	resolved := false
	for _, repo := range matched {
		if !displayableClaGroup(src, repo.ClaGroupID) {
			continue
		}
		m.recordRepository(repo.ClaGroupID, repo.Name, repo.URL)
		resolved = true
	}
	if resolved {
		return nil
	}

	for _, org := range ownerOrgs {
		for _, claGroupID := range org.claGroupIDs(sfidToClaGroups) {
			m.record(claGroupID, matchOrganization, rankExact)
		}
	}
	return nil
}

// reposNamed returns the repositories whose full name is the given path, restricted to the forge a
// known host names, or to the host itself when the host is a self-hosted one - a bare "owner/repo"
// names no host and matches either forge
func reposNamed(repos []*RepositoryRow, path, host, forge string) []*RepositoryRow {
	lowerPath := strings.ToLower(path)
	var matched []*RepositoryRow
	for _, repo := range repos {
		if strings.ToLower(repo.Name) != lowerPath {
			continue
		}
		switch {
		case forge != "":
			if repo.Type != "" && !strings.EqualFold(repo.Type, forge) {
				continue
			}
		case host != "" && hostOf(repo.URL) != host:
			continue
		}
		matched = append(matched, repo)
	}
	return matched
}

// orgsOnHost drops the organizations the host of a pasted URL rules out - the ones of another forge
// when the host names one, the ones of another host when it does not
func orgsOnHost(orgs []*OrgRow, host, forge string) []*OrgRow {
	if host == "" {
		return orgs
	}
	matched := make([]*OrgRow, 0, len(orgs))
	for _, org := range orgs {
		if forge != "" {
			if org.Source != "" && !strings.EqualFold(org.Source, forge) {
				continue
			}
		} else if hostOf(orgURL(org)) != host {
			continue
		}
		matched = append(matched, org)
	}
	return matched
}

// displayableClaGroup reports whether the CLA Group has a record or a mapping to show - a repository
// pointing at a deleted CLA Group resolves to nothing and must not suppress the organization match
func displayableClaGroup(src *sources, claGroupID string) bool {
	if claGroupID == "" {
		return false
	}
	for _, claGroup := range src.claGroups {
		if claGroup.ClaGroupID == claGroupID {
			return true
		}
	}
	for _, mapping := range src.mappings {
		if mapping.ClaGroupID == claGroupID {
			return true
		}
	}
	return false
}

// orgsNamed returns the organizations whose name is the given name, compared case-insensitively
func orgsNamed(orgs []*OrgRow, name string) []*OrgRow {
	var matched []*OrgRow
	for _, org := range orgs {
		if org.Name != "" && strings.EqualFold(org.Name, name) {
			matched = append(matched, org)
		}
	}
	return matched
}

func organizationNames(orgs []*OrgRow) []string {
	seen := map[string]bool{}
	names := make([]string, 0, len(orgs))
	for _, org := range orgs {
		if !seen[org.Name] {
			seen[org.Name] = true
			names = append(names, org.Name)
		}
	}
	return names
}

// claGroupIDs returns the CLA Groups the organization is linked to - Gerrit instances reference the
// CLA Group directly, while a GitHub organization or GitLab group references it by project SFID, by
// the CLA Group its new repositories are auto-enabled into, or by both
func (o *OrgRow) claGroupIDs(sfidToClaGroups map[string][]string) []string {
	if o.ClaGroupID != "" {
		return []string{o.ClaGroupID}
	}
	mapped := sfidToClaGroups[o.ProjectSFID]
	if o.AutoEnabledClaGroupID == "" {
		return mapped
	}
	for _, claGroupID := range mapped {
		if claGroupID == o.AutoEnabledClaGroupID {
			return mapped
		}
	}
	return append(append(make([]string, 0, len(mapped)+1), mapped...), o.AutoEnabledClaGroupID)
}

func indexProjectSFIDs(mappings []*ProjectMappingRow) map[string][]string {
	index := map[string][]string{}
	for _, mapping := range mappings {
		if mapping.ProjectSFID != "" && mapping.ClaGroupID != "" {
			index[mapping.ProjectSFID] = append(index[mapping.ProjectSFID], mapping.ClaGroupID)
		}
	}
	return index
}

func indexOrgs(orgs []*OrgRow, sfidToClaGroups map[string][]string) map[string][]models.ClaSearchOrg {
	index := map[string][]models.ClaSearchOrg{}
	for _, org := range orgs {
		for _, claGroupID := range org.claGroupIDs(sfidToClaGroups) {
			index[claGroupID] = append(index[claGroupID], models.ClaSearchOrg{Name: org.Name, Source: org.Source, URL: orgURL(org)})
		}
	}
	return index
}

// orgURL is the organization URL, derived for a GitHub organization - the github-orgs records carry none
func orgURL(org *OrgRow) string {
	if org.URL == "" && org.Source == sourceGitHub && org.Name != "" {
		return "https://github.com/" + org.Name
	}
	return org.URL
}

func buildList(searchTerm string, limit int64, matches map[string]*match, src *sources, orgsByClaGroup map[string][]models.ClaSearchOrg) *models.ClaSearchList {
	claGroupByID := map[string]*ClaGroupRow{}
	for _, claGroup := range src.claGroups {
		claGroupByID[claGroup.ClaGroupID] = claGroup
	}
	mappingsByClaGroup := map[string][]*ProjectMappingRow{}
	for _, mapping := range src.mappings {
		mappingsByClaGroup[mapping.ClaGroupID] = append(mappingsByClaGroup[mapping.ClaGroupID], mapping)
	}

	results := make([]models.ClaSearchResult, 0, len(matches))
	for claGroupID, m := range matches {
		result := buildResult(claGroupID, m, claGroupByID[claGroupID], mappingsByClaGroup[claGroupID], orgsByClaGroup[claGroupID])
		// a CLA Group with neither a record nor a mapping - a deleted one still referenced by an
		// organization - has nothing to display
		if result.ClaGroupName == "" && result.ProjectName == "" {
			continue
		}
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool {
		if ri, rj := matches[results[i].ClaGroupID].rank, matches[results[j].ClaGroupID].rank; ri != rj {
			return ri < rj
		}
		if a, b := displayName(results[i]), displayName(results[j]); a != b {
			return a < b
		}
		return results[i].ClaGroupID < results[j].ClaGroupID
	})

	truncated := int64(len(results)) > limit
	if truncated {
		results = results[:limit]
	}

	return &models.ClaSearchList{
		SearchTerm:  searchTerm,
		ResultCount: int64(len(results)),
		Truncated:   truncated,
		Results:     results,
	}
}

func buildResult(claGroupID string, m *match, claGroup *ClaGroupRow, mappings []*ProjectMappingRow, orgs []models.ClaSearchOrg) models.ClaSearchResult {
	result := models.ClaSearchResult{
		ClaGroupID:            claGroupID,
		MatchTypes:            sortedKeys(m.types),
		MatchedRepositoryName: m.repoName,
		MatchedRepositoryURL:  m.repoURL,
		Organizations:         sortOrgs(orgs),
	}
	if claGroup != nil {
		result.ClaGroupName = claGroup.Name
		result.ProjectExternalID = claGroup.ExternalID
		result.IclaEnabled = enabledOrDefault(claGroup.IclaEnabled)
		result.CclaEnabled = enabledOrDefault(claGroup.CclaEnabled)
	}

	// A foundation-level CLA Group is marked by a mapping whose ProjectSFID equals its
	// FoundationSFID (the projects_cla_groups convention) and resolves to its foundation; a single
	// project-level mapping resolves to that project. Several project-level mappings with no
	// foundation marker are left unresolved rather than picking an arbitrary one.
	for _, mapping := range mappings {
		if result.ClaGroupName == "" {
			result.ClaGroupName = mapping.ClaGroupName
		}
		if result.FoundationSFID == "" {
			result.FoundationSFID = mapping.FoundationSFID
		}
		if mapping.FoundationSFID != "" && mapping.FoundationSFID == mapping.ProjectSFID {
			result.ProjectSFID, result.ProjectName = mapping.FoundationSFID, mapping.FoundationName
			return result
		}
	}
	if len(mappings) == 1 {
		result.ProjectSFID, result.ProjectName = mappings[0].ProjectSFID, mappings[0].ProjectName
	}
	return result
}

// enabledOrDefault reads a CLA type flag, a missing attribute meaning enabled - the Pynamo
// default=True the v1 CLA Group reader also honours
func enabledOrDefault(enabled *bool) bool {
	return enabled == nil || *enabled
}

func displayName(result models.ClaSearchResult) string {
	if result.ProjectName != "" {
		return strings.ToLower(result.ProjectName)
	}
	return strings.ToLower(result.ClaGroupName)
}

func sortOrgs(orgs []models.ClaSearchOrg) []models.ClaSearchOrg {
	if orgs == nil {
		return []models.ClaSearchOrg{}
	}
	sort.Slice(orgs, func(i, j int) bool {
		if orgs[i].Source != orgs[j].Source {
			return orgs[i].Source < orgs[j].Source
		}
		return orgs[i].Name < orgs[j].Name
	})
	return orgs
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// rankOf scores how well the already lower-cased term matches value - lower is better, -1 is no match
func rankOf(value, term string) int {
	value = strings.ToLower(value)
	switch {
	case value == "" || term == "":
		return -1
	case value == term:
		return rankExact
	case strings.HasPrefix(value, term):
		return rankPrefix
	case strings.Contains(value, term):
		return rankSubstring
	default:
		return -1
	}
}

func bestRank(term string, values ...string) int {
	best := -1
	for _, value := range values {
		if rank := rankOf(value, term); rank >= 0 && (best < 0 || rank < best) {
			best = rank
		}
	}
	return best
}

func hostOf(rawURL string) string {
	parsed, err := url.Parse(strings.ToLower(rawURL))
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

// repositoryPath derives the full repository name the term addresses - the path of a pasted
// repository URL, or the term itself when it looks like an "owner/repo" path - together with the
// host the URL names, and is empty when the term addresses no repository
func repositoryPath(term string) (string, string) {
	path, host := term, ""
	if strings.Contains(term, "://") {
		parsed, err := url.Parse(term)
		if err != nil || parsed.Hostname() == "" {
			return "", ""
		}
		path, host = parsed.Path, strings.ToLower(parsed.Hostname())
	}
	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimPrefix(path, "groups/")
	if !strings.Contains(path, "/") || strings.ContainsAny(path, " \t") {
		return "", ""
	}
	return path, host
}

// nameVariants are the repository names looked up on the case-preserved repository-name-index GSI
func nameVariants(path string) []string {
	if lower := strings.ToLower(path); lower != path {
		return []string{path, lower}
	}
	return []string{path}
}

// urlSignal is the part of an organization URL that identifies the organization - the host of a
// shared forge is the same for every organization hosted on it and carries no signal, while the
// host of a self-hosted instance such as a Gerrit server is the only thing that does
func urlSignal(rawURL string) string {
	parsed, err := url.Parse(strings.ToLower(rawURL))
	if err != nil {
		return strings.ToLower(rawURL)
	}
	if forgeHosts[parsed.Hostname()] != "" {
		return strings.Trim(parsed.Path, "/")
	}
	return parsed.Hostname() + parsed.Path
}
