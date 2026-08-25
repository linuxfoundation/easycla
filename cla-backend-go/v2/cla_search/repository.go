// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_search

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/repositories"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// Scan segment counts, sized so every table is covered by one round of parallel 1MB scan pages -
// the CLA Group table carries the embedded CLA document bodies and is by far the largest
const (
	claGroupScanSegments       = 8
	projectMappingScanSegments = 2
	orgScanSegments            = 1

	// the number of DynamoDB calls a single search can have in flight
	searchConcurrency = claGroupScanSegments + projectMappingScanSegments + 3*orgScanSegments + 2
)

// ClaGroupRow is a CLA Group record, projected to the fields the search results carry. The CLA type
// flags are pointers because a missing attribute means true - the Pynamo default the v1 reader keeps.
type ClaGroupRow struct {
	ClaGroupID  string `dynamodbav:"project_id"`
	Name        string `dynamodbav:"project_name"`
	ExternalID  string `dynamodbav:"project_external_id"`
	IclaEnabled *bool  `dynamodbav:"project_icla_enabled"`
	CclaEnabled *bool  `dynamodbav:"project_ccla_enabled"`
}

// ProjectMappingRow is a projects-cla-groups mapping record, projected to the fields the search results carry
type ProjectMappingRow struct {
	ClaGroupID     string `dynamodbav:"cla_group_id"`
	ClaGroupName   string `dynamodbav:"cla_group_name"`
	ProjectSFID    string `dynamodbav:"project_sfid"`
	ProjectName    string `dynamodbav:"project_name"`
	FoundationSFID string `dynamodbav:"foundation_sfid"`
	FoundationName string `dynamodbav:"foundation_name"`
}

// OrgRow is a repository-hosting organization - a GitHub organization, a GitLab group or a Gerrit instance
type OrgRow struct {
	Name                  string
	URL                   string
	Source                string
	ProjectSFID           string
	ClaGroupID            string
	AutoEnabledClaGroupID string
}

// RepositoryRow is the repository a pasted URL or "owner/repo" path resolved to
type RepositoryRow struct {
	Name       string `dynamodbav:"repository_name"`
	URL        string `dynamodbav:"repository_url"`
	Type       string `dynamodbav:"repository_type"`
	ClaGroupID string `dynamodbav:"repository_project_id"`
}

// Repository interface defines the data access methods for the CLA Group search module
type Repository interface {
	GetClaGroups(ctx context.Context) ([]*ClaGroupRow, error)
	GetProjectMappings(ctx context.Context) ([]*ProjectMappingRow, error)
	GetGithubOrgs(ctx context.Context) ([]*OrgRow, error)
	GetGitlabOrgs(ctx context.Context) ([]*OrgRow, error)
	GetGerritInstances(ctx context.Context) ([]*OrgRow, error)
	GetRepositoriesByName(ctx context.Context, names []string) ([]*RepositoryRow, error)
	GetRepositoriesByOrganization(ctx context.Context, organizationNames []string) ([]*RepositoryRow, error)
}

type repository struct {
	dynamoDBClient          *dynamodb.DynamoDB
	claGroupTableName       string
	projectMappingTableName string
	githubOrgTableName      string
	gitlabOrgTableName      string
	gerritTableName         string
	repositoryTableName     string
}

// NewRepository creates a new instance of the CLA Group search repository, with the scanned tables
// served from the in-process cache
func NewRepository(awsSession *session.Session, stage string) Repository {
	return newCachedRepository(newScanRepository(awsSession, stage), cacheTTL())
}

func newScanRepository(awsSession *session.Session, stage string) Repository {
	// a search fans out to more concurrent DynamoDB calls than the default two idle connections
	// per host can serve, which would leave most of them paying for a fresh TLS handshake
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		MaxIdleConns:        searchConcurrency,
		MaxIdleConnsPerHost: searchConcurrency,
		IdleConnTimeout:     90 * time.Second,
	}

	return repository{
		dynamoDBClient:          dynamodb.New(awsSession.Copy(&aws.Config{HTTPClient: &http.Client{Transport: transport}})),
		claGroupTableName:       fmt.Sprintf("cla-%s-projects", stage),
		projectMappingTableName: fmt.Sprintf("cla-%s-projects-cla-groups", stage),
		githubOrgTableName:      fmt.Sprintf("cla-%s-github-orgs", stage),
		gitlabOrgTableName:      fmt.Sprintf("cla-%s-gitlab-orgs", stage),
		gerritTableName:         fmt.Sprintf("cla-%s-gerrit-instances", stage),
		repositoryTableName:     fmt.Sprintf("cla-%s-repositories", stage),
	}
}

// enabledFilter keeps the disabled records - organizations unlinked from EasyCLA and repositories
// no longer covered by a CLA Group - out of the search, matching the convention of the
// github_organizations repository
func enabledFilter() expression.ConditionBuilder {
	return expression.Name("enabled").Equal(expression.Value(true))
}

func (repo repository) GetClaGroups(ctx context.Context) ([]*ClaGroupRow, error) {
	var rows []*ClaGroupRow
	err := repo.scan(ctx, repo.claGroupTableName, claGroupScanSegments, nil,
		[]string{"project_id", "project_name", "project_external_id", "project_icla_enabled", "project_ccla_enabled"}, &rows)
	return rows, err
}

func (repo repository) GetProjectMappings(ctx context.Context) ([]*ProjectMappingRow, error) {
	var rows []*ProjectMappingRow
	err := repo.scan(ctx, repo.projectMappingTableName, projectMappingScanSegments, nil,
		[]string{"cla_group_id", "cla_group_name", "project_sfid", "project_name", "foundation_sfid", "foundation_name"}, &rows)
	return rows, err
}

type orgDBRow struct {
	Name                  string `dynamodbav:"organization_name"`
	URL                   string `dynamodbav:"organization_url"`
	ProjectSFID           string `dynamodbav:"project_sfid"`
	AutoEnabledClaGroupID string `dynamodbav:"auto_enabled_cla_group_id"`
}

func (repo repository) GetGithubOrgs(ctx context.Context) ([]*OrgRow, error) {
	return repo.scanOrgs(ctx, repo.githubOrgTableName, sourceGitHub)
}

func (repo repository) GetGitlabOrgs(ctx context.Context) ([]*OrgRow, error) {
	return repo.scanOrgs(ctx, repo.gitlabOrgTableName, sourceGitLab)
}

func (repo repository) scanOrgs(ctx context.Context, tableName, source string) ([]*OrgRow, error) {
	var rows []*orgDBRow
	filter := enabledFilter()
	if err := repo.scan(ctx, tableName, orgScanSegments, &filter,
		[]string{"organization_name", "organization_url", "project_sfid", "auto_enabled_cla_group_id"}, &rows); err != nil {
		return nil, err
	}
	orgs := make([]*OrgRow, 0, len(rows))
	for _, row := range rows {
		orgs = append(orgs, &OrgRow{Name: row.Name, URL: row.URL, Source: source, ProjectSFID: row.ProjectSFID,
			AutoEnabledClaGroupID: row.AutoEnabledClaGroupID})
	}
	return orgs, nil
}

type gerritDBRow struct {
	Name       string `dynamodbav:"gerrit_name"`
	URL        string `dynamodbav:"gerrit_url"`
	ClaGroupID string `dynamodbav:"project_id"`
}

func (repo repository) GetGerritInstances(ctx context.Context) ([]*OrgRow, error) {
	var rows []*gerritDBRow
	// the gerrit-instances table carries no enabled flag
	if err := repo.scan(ctx, repo.gerritTableName, orgScanSegments, nil, []string{"gerrit_name", "gerrit_url", "project_id"}, &rows); err != nil {
		return nil, err
	}
	orgs := make([]*OrgRow, 0, len(rows))
	for _, row := range rows {
		orgs = append(orgs, &OrgRow{Name: row.Name, URL: row.URL, Source: sourceGerrit, ClaGroupID: row.ClaGroupID})
	}
	return orgs, nil
}

// GetRepositoriesByName resolves the given full repository names through the repository-name-index
// GSI - an exact hash-key lookup per name, no scan
func (repo repository) GetRepositoriesByName(ctx context.Context, names []string) ([]*RepositoryRow, error) {
	return repo.queryRepositories(ctx, repositories.RepositoryNameIndex, "repository_name", names)
}

// GetRepositoriesByOrganization returns the repositories of the given organizations through the
// repository-organization-name-index GSI, which is how a repository whose stored name is not
// lower-cased is recovered from a lower-cased pasted URL
func (repo repository) GetRepositoriesByOrganization(ctx context.Context, organizationNames []string) ([]*RepositoryRow, error) {
	return repo.queryRepositories(ctx, repositories.RepositoryOrganizationNameIndex, "repository_organization_name", organizationNames)
}

// queryRepositories runs one enabled-only GSI query per key value, in parallel
func (repo repository) queryRepositories(ctx context.Context, indexName, keyAttribute string, values []string) ([]*RepositoryRow, error) {
	f := logrus.Fields{
		"functionName":   "v2.cla_search.repository.queryRepositories",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"indexName":      indexName,
		"values":         values,
	}

	var (
		mu   sync.Mutex
		rows []*RepositoryRow
	)
	group, groupCtx := errgroup.WithContext(ctx)
	for _, value := range values {
		keyValue := value
		group.Go(func() error {
			expr, err := expression.NewBuilder().
				WithKeyCondition(expression.Key(keyAttribute).Equal(expression.Value(keyValue))).
				WithFilter(enabledFilter()).
				WithProjection(expression.NamesList(expression.Name("repository_name"), expression.Name("repository_url"),
					expression.Name("repository_type"), expression.Name("repository_project_id"))).
				Build()
			if err != nil {
				log.WithFields(f).WithError(err).Warn("error building expression for the repository query")
				return err
			}

			queryInput := &dynamodb.QueryInput{
				ExpressionAttributeNames:  expr.Names(),
				ExpressionAttributeValues: expr.Values(),
				FilterExpression:          expr.Filter(),
				KeyConditionExpression:    expr.KeyCondition(),
				ProjectionExpression:      expr.Projection(),
				TableName:                 aws.String(repo.repositoryTableName),
				IndexName:                 aws.String(indexName),
			}
			for {
				results, queryErr := repo.dynamoDBClient.QueryWithContext(groupCtx, queryInput)
				if queryErr != nil {
					log.WithFields(f).WithError(queryErr).Warn("error querying repositories")
					return queryErr
				}

				var page []*RepositoryRow
				if unmarshalErr := dynamodbattribute.UnmarshalListOfMaps(results.Items, &page); unmarshalErr != nil {
					log.WithFields(f).WithError(unmarshalErr).Warn("error unmarshalling repositories")
					return unmarshalErr
				}

				mu.Lock()
				rows = append(rows, page...)
				mu.Unlock()

				if len(results.LastEvaluatedKey) == 0 {
					return nil
				}
				queryInput.ExclusiveStartKey = results.LastEvaluatedKey
			}
		})
	}
	if err := group.Wait(); err != nil {
		return nil, err
	}

	return rows, nil
}

// scan runs a projected full-table scan into out, which must be a pointer to a slice. The table is
// split into segments scanned in parallel so a table spanning several 1MB scan pages costs one
// round trip rather than one per page.
func (repo repository) scan(ctx context.Context, tableName string, segments int, filter *expression.ConditionBuilder, attributes []string, out interface{}) error {
	f := logrus.Fields{
		"functionName":   "v2.cla_search.repository.scan",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"tableName":      tableName,
		"segments":       segments,
	}

	if len(attributes) == 0 {
		return fmt.Errorf("no attributes to project from table %s", tableName)
	}

	names := make([]expression.NameBuilder, 0, len(attributes))
	for _, attribute := range attributes {
		names = append(names, expression.Name(attribute))
	}
	builder := expression.NewBuilder().WithProjection(expression.NamesList(names[0], names[1:]...))
	if filter != nil {
		builder = builder.WithFilter(*filter)
	}
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error building expression for the scan")
		return err
	}

	var (
		mu    sync.Mutex
		items []map[string]*dynamodb.AttributeValue
	)
	group, groupCtx := errgroup.WithContext(ctx)
	for segment := 0; segment < segments; segment++ {
		scanInput := &dynamodb.ScanInput{
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			FilterExpression:          expr.Filter(),
			ProjectionExpression:      expr.Projection(),
			TableName:                 aws.String(tableName),
		}
		if segments > 1 {
			scanInput.Segment = aws.Int64(int64(segment))
			scanInput.TotalSegments = aws.Int64(int64(segments))
		}
		group.Go(func() error {
			for {
				results, scanErr := repo.dynamoDBClient.ScanWithContext(groupCtx, scanInput)
				if scanErr != nil {
					log.WithFields(f).WithError(scanErr).Warn("error scanning table")
					return scanErr
				}
				mu.Lock()
				items = append(items, results.Items...)
				mu.Unlock()
				if len(results.LastEvaluatedKey) == 0 {
					return nil
				}
				scanInput.ExclusiveStartKey = results.LastEvaluatedKey
			}
		})
	}
	if err := group.Wait(); err != nil {
		return err
	}
	return dynamodbattribute.UnmarshalListOfMaps(items, out)
}
