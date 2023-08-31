// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package metrics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	project_service "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/token"
	"github.com/imroc/req"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// errors
var (
	ErrMetricNotFound = errors.New("metric not found")
)

// LfMembers contains names of LF members
var (
	LfMembers                     = make(map[string]interface{})
	processedSFProjectsMembership = make(map[string]interface{})
)

// index
const (
	IndexMetricTypeSalesforceID = "metric-type-salesforce-id-index"
)

// Repository provides methods for calculation,storage and retrieval of metrics
type Repository interface {
	CalculateAndSaveMetrics() error
	GetClaManagerDistribution() (*ClaManagersDistribution, error)
	GetTotalCountMetrics() (*TotalCountMetrics, error)
	GetCompanyMetrics() ([]*CompanyMetric, error)
	GetProjectMetrics(pageSize int64, nextKey string) ([]*ProjectMetric, string, error)
	GetCompanyMetric(companyID string) (*CompanyMetric, error)
	GetProjectMetric(projectID string) (*ProjectMetric, error)
	GetProjectMetricBySalesForceID(salesforceID string) ([]*ProjectMetric, error)
	ListCompanyProjectMetrics(companyID string) ([]*CompanyProjectMetric, error)
}

type repo struct {
	metricTableName       string
	dynamoDBClient        *dynamodb.DynamoDB
	stage                 string
	apiGatewayURL         string
	projectsClaGroupsRepo projects_cla_groups.Repository
}

// NewRepository creates new metrics repository
func NewRepository(awsSession *session.Session, stage string, apiGwURL string, pcgRepo projects_cla_groups.Repository) Repository {
	return &repo{
		dynamoDBClient:        dynamodb.New(awsSession),
		metricTableName:       fmt.Sprintf("cla-%s-metrics", stage),
		stage:                 stage,
		apiGatewayURL:         apiGwURL,
		projectsClaGroupsRepo: pcgRepo,
	}
}

// SignatureType constants
const (
	IclaSignature = iota
	CclaSignature
	EmployeeSignature
	InvalidSignature
)

// MetricType constants and ID constants
const (
	MetricTypeTotalCount             = "total_count"
	MetricTypeCompany                = "company"
	MetricTypeProject                = "project"
	MetricTypeCompanyProject         = "company_project"
	MetricTypeClaManagerDistribution = "cla_manager_distribution" // nolint G101: Potential hardcoded credentials (gosec)

	IDTotalCount             = "total_count"
	IDClaManagerDistribution = "cla_manager_distribution" // nolint G101: Potential hardcoded credentials (gosec)
)

func newMetrics() *Metrics {
	return &Metrics{
		TotalCountMetrics:       newTotalCountMetrics(),
		CompanyMetrics:          newCompanyMetrics(),
		ProjectMetrics:          newProjectMetrics(),
		CompanyProjectMetrics:   newCompanyProjectMetrics(),
		ClaManagersDistribution: &ClaManagersDistribution{},
	}
}

func newTotalCountMetrics() *TotalCountMetrics {
	return &TotalCountMetrics{
		CorporateContributorsCount:        0,
		corporateContributors:             make(map[string]interface{}),
		IndividualContributorsCount:       0,
		individualContributors:            make(map[string]interface{}),
		ClaManagersCount:                  0,
		claManagers:                       make(map[string]interface{}),
		ContributorsCount:                 0,
		contributors:                      make(map[string]interface{}),
		CompaniesProjectContributionCount: 0,
		companiesProjectContribution:      make(map[string]interface{}),
	}
}

func newCompanyMetric() *CompanyMetric {
	return &CompanyMetric{
		ProjectCount:               0,
		CorporateContributorsCount: 0,
		corporateContributors:      make(map[string]interface{}),
		ClaManagersCount:           0,
		claManagers:                make(map[string]interface{}),
	}
}

// CompanyMetrics contain collection of all company metrics
type CompanyMetrics struct {
	CompanyMetrics map[string]*CompanyMetric
}

func newCompanyMetrics() *CompanyMetrics {
	return &CompanyMetrics{
		CompanyMetrics: make(map[string]*CompanyMetric),
	}
}

func newProjectMetric() *ProjectMetric {
	return &ProjectMetric{
		CompaniesCount:              0,
		companies:                   make(map[string]interface{}),
		ClaManagersCount:            0,
		claManagers:                 make(map[string]interface{}),
		CorporateContributorsCount:  0,
		corporateContributors:       make(map[string]interface{}),
		IndividualContributorsCount: 0,
		individualContributors:      make(map[string]interface{}),
		TotalContributorsCount:      0,
		RepositoriesCount:           0,
		ExternalProjectID:           "",
		ProjectName:                 "",
	}
}

// ProjectMetrics contain collection of all project metrics
type ProjectMetrics struct {
	ProjectMetrics map[string]*ProjectMetric
}

func newProjectMetrics() *ProjectMetrics {
	return &ProjectMetrics{
		ProjectMetrics: make(map[string]*ProjectMetric),
	}
}

// CompanyProjectMetric contain metrics for company-project pair
type CompanyProjectMetric struct {
	CompanyID         string `json:"company_id"`
	ProjectID         string `json:"project_id"`
	ClaGroupName      string `json:"cla_group_name"`
	ProjectName       string `json:"project_name"`
	CompanyName       string `json:"company_name"`
	ClaManagersCount  int64  `json:"cla_managers_count"`
	ContributorsCount int64  `json:"contributors_count"`
	ProjectSFID       string `json:"project_sfid"`
	claManagers       map[string]interface{}
	contributors      map[string]interface{}
}

// CompanyProjectMetrics contain collection of company-project metric
type CompanyProjectMetrics struct {
	CompanyProjectMetrics map[string]*CompanyProjectMetric
}

func newCompanyProjectMetrics() *CompanyProjectMetrics {
	return &CompanyProjectMetrics{
		CompanyProjectMetrics: make(map[string]*CompanyProjectMetric),
	}
}

func (pcm *CompanyProjectMetrics) getCompanyProjectMetric(projectID, companyID string) *CompanyProjectMetric {
	id := fmt.Sprintf("%s#%s", companyID, projectID)
	m, ok := pcm.CompanyProjectMetrics[id]
	if !ok {
		m = &CompanyProjectMetric{
			CompanyID:    companyID,
			ProjectID:    projectID,
			claManagers:  make(map[string]interface{}),
			contributors: make(map[string]interface{}),
		}
		pcm.CompanyProjectMetrics[id] = m
	}
	return m
}

func increaseCountIfNotPresent(cacheData map[string]interface{}, count *int64, key string) {
	if _, ok := cacheData[key]; !ok {
		cacheData[key] = nil
		*count++
	}
}

func (cm *CompanyMetric) toModel() *models.CompanyMetric {
	return &models.CompanyMetric{
		ClaManagersCount:           cm.ClaManagersCount,
		CompanyName:                cm.CompanyName,
		CorporateContributorsCount: cm.CorporateContributorsCount,
		CreatedAt:                  cm.CreatedAt,
		ID:                         cm.ID,
		ProjectsCount:              cm.ProjectCount,
	}
}

func (pm *ProjectMetric) toModel() *models.ProjectMetric {
	return &models.ProjectMetric{
		ClaManagersCount:            pm.ClaManagersCount,
		CompaniesCount:              pm.CompaniesCount,
		CorporateContributorsCount:  pm.CorporateContributorsCount,
		CreatedAt:                   pm.CreatedAt,
		ID:                          pm.ID,
		IndividualContributorsCount: pm.IndividualContributorsCount,
		RepositoriesCount:           pm.RepositoriesCount,
		TotalContributorsCount:      pm.TotalContributorsCount,
		ExternalProjectID:           pm.ExternalProjectID,
		ProjectName:                 pm.ProjectName,
	}
}

func (cpm *CompanyProjectMetric) toModel() *models.CompanyProjectMetric {
	return &models.CompanyProjectMetric{
		ClaManagersCount:  cpm.ClaManagersCount,
		CompanyID:         cpm.CompanyID,
		ContributorsCount: cpm.ContributorsCount,
		ProjectID:         cpm.ProjectID,
		ProjectName:       cpm.ProjectName,
		CompanyName:       cpm.CompanyName,
		ProjectSFID:       cpm.ProjectSFID,
		ClaGroupName:      cpm.ClaGroupName,
	}
}

func (tcm *TotalCountMetrics) toModel() *models.TotalCountMetrics {
	return &models.TotalCountMetrics{
		ProjectsCount:                     tcm.ProjectsCount,
		IndividualContributorsCount:       tcm.IndividualContributorsCount,
		CompaniesProjectContributionCount: tcm.CompaniesProjectContributionCount,
		ContributorsCount:                 tcm.ContributorsCount,
		RepositoriesCount:                 tcm.RepositoriesCount,
		CreatedAt:                         tcm.CreatedAt,
		// Removed the following metrics per stakeholder request
		//CorporateContributorsCount:        tcm.CorporateContributorsCount,
		//ClaManagersCount:                  tcm.ClaManagersCount,
		//GerritRepositoriesCount:           tcm.GerritRepositoriesCount,
		//CompaniesCount:                    tcm.CompaniesCount,
		//GithubRepositoriesCount:           tcm.GithubRepositoriesCount,
		//LfMembersCLACount:                 tcm.LfMembersCLACount,
		//NonLfMembersCLACount:              tcm.NonLfMembersCLACount,
	}
}

func companiesToModel(in []*CompanyMetric) []*models.CompanyMetric {
	out := make([]*models.CompanyMetric, 0)
	for _, cm := range in {
		out = append(out, cm.toModel())
	}
	return out
}

func projectsToModel(in []*ProjectMetric) []*models.ProjectMetric {
	out := make([]*models.ProjectMetric, 0)
	for _, pm := range in {
		out = append(out, pm.toModel())
	}
	return out
}

func signatureType(sig *ItemSignature) int {
	if sig.SignatureType == "ccla" && sig.SignatureReferenceType == "company" {
		return CclaSignature
	}
	if sig.SignatureType == "cla" {
		if sig.SignatureUserCompanyID != "" {
			return EmployeeSignature
		}
		return IclaSignature
	}
	return InvalidSignature
}

func (m *Metrics) processSignature(sig *ItemSignature, usersCache map[string]*ItemUser) {
	sigType := signatureType(sig)
	if sigType == InvalidSignature {
		log.Printf("Warn: invalid signature: %v\n", sig)
		return
	}
	m.CompanyMetrics.processSignature(sig, sigType, usersCache)
	m.TotalCountMetrics.processSignature(sig, sigType, usersCache)
	m.ProjectMetrics.processSignature(sig, sigType, usersCache)
	m.CompanyProjectMetrics.processSignature(sig, sigType, usersCache)
}

// calculate total count metrics fields as follows
// number of ccla signed
// ccla signed by lf members
// ccla signed by non-lf members
// corporate contributors
// individual contributors
// total contributors
func (tcm *TotalCountMetrics) processSignature(sig *ItemSignature, sigType int, usersCache map[string]*ItemUser) {
	switch sigType {
	case CclaSignature:
		for _, claManagerLfusername := range sig.SignatureACL {
			if _, ok := usersCache[claManagerLfusername]; ok {
				// only increase cla manager count if user is present in database
				increaseCountIfNotPresent(tcm.claManagers, &tcm.ClaManagersCount, claManagerLfusername)
			}
		}
		companyID := sig.SignatureReferenceID
		companyName := sig.SignatureReferenceName
		key := fmt.Sprintf("%s#%s", companyID, sig.SignatureProjectID)
		increaseCountIfNotPresent(tcm.companiesProjectContribution, &tcm.CompaniesProjectContributionCount, key)
		if _, ok := LfMembers[companyName]; ok {
			tcm.LfMembersCLACount++
		} else {
			tcm.NonLfMembersCLACount++
		}
		tcm.CLAsSignedCount++
	case EmployeeSignature:
		userID := sig.SignatureReferenceID
		increaseCountIfNotPresent(tcm.corporateContributors, &tcm.CorporateContributorsCount, userID)
		increaseCountIfNotPresent(tcm.contributors, &tcm.ContributorsCount, userID)
	case IclaSignature:
		userID := sig.SignatureReferenceID
		increaseCountIfNotPresent(tcm.individualContributors, &tcm.IndividualContributorsCount, userID)
		increaseCountIfNotPresent(tcm.contributors, &tcm.ContributorsCount, userID)
		tcm.CLAsSignedCount++
	}
}

// calculate number of cla-managers of company for particular project
// calculate number of contributors of company for particular project
func (pcm *CompanyProjectMetrics) processSignature(sig *ItemSignature, sigType int, usersCache map[string]*ItemUser) {
	projectID := sig.SignatureProjectID
	switch sigType {
	case CclaSignature:
		companyID := sig.SignatureReferenceID
		m := pcm.getCompanyProjectMetric(projectID, companyID)
		for _, claManagerLfusername := range sig.SignatureACL {
			if _, ok := usersCache[claManagerLfusername]; ok {
				// only increase cla manager count if user is present in database
				increaseCountIfNotPresent(m.claManagers, &m.ClaManagersCount, claManagerLfusername)
			}
		}
	case EmployeeSignature:
		companyID := sig.SignatureUserCompanyID
		userID := sig.SignatureReferenceID
		m := pcm.getCompanyProjectMetric(projectID, companyID)
		increaseCountIfNotPresent(m.contributors, &m.ContributorsCount, userID)
	}
}

// calculate company metrics fields as follows
// corporate contributors of the company
// cla-managers of the company
func (cm *CompanyMetrics) processSignature(sig *ItemSignature, sigType int, usersCache map[string]*ItemUser) {
	switch sigType {
	case CclaSignature:
		companyName := sig.SignatureReferenceName
		companyID := sig.SignatureReferenceID
		m, ok := cm.CompanyMetrics[companyID]
		if !ok {
			log.Warnf("company id=[%s]:name=[%s] does not exist in companies table but ccla signature [%s] for it is present", companyID, companyName, sig.SignatureID)
			// skipping processing signature as company is not present in database
			return
		}
		m.ProjectCount++
		for _, claManagerLfusername := range sig.SignatureACL {
			if _, ok := usersCache[claManagerLfusername]; ok {
				// only increase cla manager count if user is present in database
				increaseCountIfNotPresent(m.claManagers, &m.ClaManagersCount, claManagerLfusername)
			}
		}
	case EmployeeSignature:
		companyID := sig.SignatureUserCompanyID
		m, ok := cm.CompanyMetrics[companyID]
		if !ok {
			log.Warnf("company id=[%s] does not exist in companies table but employee signature [%s] for it is present", companyID, sig.SignatureID)
			// skipping processing signature as company is not present in database
			return
		}
		userID := sig.SignatureReferenceID
		increaseCountIfNotPresent(m.corporateContributors, &m.CorporateContributorsCount, userID)
	}
}

// calculate project metrics fields as follows
// companies count for project
// cla-managers in project
// corporate contributors in project
// individual contributors in project
// total contributors in project
func (pm *ProjectMetrics) processSignature(sig *ItemSignature, sigType int, usersCache map[string]*ItemUser) {
	projectID := sig.SignatureProjectID
	m, ok := pm.ProjectMetrics[projectID]
	if !ok {
		log.Warnf("project id=[%s] does not exist in projects table but signature [%s] for it is present", projectID, sig.SignatureID)
		// skipping processing signature as project is not present in database
		return
	}
	switch sigType {
	case CclaSignature:
		companyID := sig.SignatureReferenceID
		increaseCountIfNotPresent(m.companies, &m.CompaniesCount, companyID)
		for _, claManagerLfusername := range sig.SignatureACL {
			if _, ok := usersCache[claManagerLfusername]; ok {
				// only increase cla manager count if user is present in database
				increaseCountIfNotPresent(m.claManagers, &m.ClaManagersCount, claManagerLfusername)
			}
		}
	case EmployeeSignature:
		userID := sig.SignatureReferenceID
		increaseCountIfNotPresent(m.corporateContributors, &m.CorporateContributorsCount, userID)
	case IclaSignature:
		userID := sig.SignatureReferenceID
		increaseCountIfNotPresent(m.individualContributors, &m.IndividualContributorsCount, userID)
	}
	m.TotalContributorsCount = m.IndividualContributorsCount + m.CorporateContributorsCount
}

func (pm *ProjectMetrics) processRepositories(repo *ItemRepository) {
	projectID := repo.RepositoryProjectID
	m, ok := pm.ProjectMetrics[projectID]
	if !ok {
		m = newProjectMetric()
		pm.ProjectMetrics[projectID] = m
	}
	m.RepositoriesCount++
}

func (pm *ProjectMetrics) processGerritInstance(gi *ItemGerritInstance) {
	projectID := gi.ProjectID
	m, ok := pm.ProjectMetrics[projectID]
	if !ok {
		m = newProjectMetric()
		pm.ProjectMetrics[projectID] = m
	}
	m.RepositoriesCount++
}

func (pm *ProjectMetrics) processProjectItem(project *ItemProject, apiGatewayURL string) {
	m, ok := pm.ProjectMetrics[project.ProjectID]
	if !ok {
		m = newProjectMetric()
		pm.ProjectMetrics[project.ProjectID] = m
	}
	m.ExternalProjectID = project.ProjectExternalID
	m.SalesforceID = project.ProjectExternalID
	m.ProjectName = project.ProjectName

	if project.ProjectExternalID != "" {
		_, ok := processedSFProjectsMembership[project.ProjectExternalID]
		if !ok {
			processedSFProjectsMembership[project.ProjectExternalID] = nil
			cacheProjectMembership(project.ProjectExternalID, apiGatewayURL)
		}
	}
}

func (cm *CompanyMetrics) processCompanyItem(company *ItemCompany) {
	m, ok := cm.CompanyMetrics[company.CompanyID]
	if !ok {
		m = newCompanyMetric()
		cm.CompanyMetrics[company.CompanyID] = m
	}
	m.CompanyName = company.CompanyName
}

func cacheProjectMembership(externalProjectID, apiGatewayURL string) {
	url := apiGatewayURL + "/project-service/v1/projects/" + externalProjectID + "/members"

	// Grab our token
	authToken, err := token.GetToken()
	if err != nil {
		log.Error("unable to get token", err)
		return
	}

	// Use Req object to initiate requests.
	r := req.New()
	authHeader := req.Header{
		"Authorization": "Bearer " + authToken,
		"Accept":        "application/json",
	}
	resp, err := r.Get(url, authHeader)
	if err != nil {
		log.Warnf("project fetch failed with error: %+v", err)
		return
	}

	log.Debugf("Project %s service query response code: %d", externalProjectID, resp.Response().StatusCode)
	if resp.Response().StatusCode < 200 || resp.Response().StatusCode > 299 {
		log.Warnf("unable to query project service - received error response: %s", resp.String())
		return
	}

	var response []struct {
		Name string `json:"name"`
	}

	jsonErr := resp.ToJSON(&response)
	if jsonErr != nil {
		log.Warnf("error unmarshalling response to JSON, error: %+v", jsonErr)
		return
	}
	for _, r := range response {
		LfMembers[r.Name] = nil
	}
}

func calculateClaManagerDistribution(cm *CompanyMetrics) *ClaManagersDistribution {
	var cmd ClaManagersDistribution
	for _, companyMetric := range cm.CompanyMetrics {
		switch companyMetric.ClaManagersCount {
		case 1:
			cmd.OneClaManager++
		case 2:
			cmd.TwoClaManager++
		case 3:
			cmd.ThreeClaManager++
		default:
			if companyMetric.ClaManagersCount >= 4 {
				cmd.FourOrMoreClaManager++
			}
		}
	}
	return &cmd
}

func (repo *repo) processSignaturesTable(metrics *Metrics, usersCache map[string]*ItemUser) error {
	log.Println("processing signatures table")
	filter := expression.Name("signature_signed").Equal(expression.Value(true)).
		And(expression.Name("signature_approved").Equal(expression.Value(true)))
	projection := expression.NamesList(
		expression.Name("signature_id"), // signature id
		expression.Name("signature_reference_id"),
		expression.Name("signature_reference_name"), // Added to support simplified UX queries
		expression.Name("signature_acl"),
		expression.Name("signature_user_ccla_company_id"), // reference to the company
		expression.Name("signature_type"),                 // ccla or cla
		expression.Name("signature_reference_type"),       // user or company
		expression.Name("signature_project_id"),           // project id
	)
	signatureTableName := fmt.Sprintf("cla-%s-signatures", repo.stage)
	var sigs []*ItemSignature
	err := repo.scanTable(signatureTableName, projection, &filter, &sigs)
	if err != nil {
		return err
	}
	for _, sig := range sigs {
		metrics.processSignature(sig, usersCache)
	}
	return nil
}

func (repo *repo) processRepositoriesTable(metrics *Metrics) error {
	log.Println("processing repositories table")
	projection := expression.NamesList(
		expression.Name("repository_project_id"),
		expression.Name("enabled"),
	)
	repositoriesTableName := fmt.Sprintf("cla-%s-repositories", repo.stage)
	var repos []*ItemRepository
	err := repo.scanTable(repositoriesTableName, projection, nil, &repos)
	if err != nil {
		return err
	}
	for _, r := range repos {
		if r.Enabled {
			metrics.TotalCountMetrics.GithubRepositoriesEnabledCount++
		}
		metrics.TotalCountMetrics.GithubRepositoriesCount++
		metrics.ProjectMetrics.processRepositories(r)
	}
	return nil
}

func (repo *repo) processGerritInstancesTable(metrics *Metrics) error {
	log.Println("processing gerrit instances table")
	projection := expression.NamesList(
		expression.Name("project_id"),
	)
	var gerritInstances []*ItemGerritInstance
	gerritInstancesTableName := fmt.Sprintf("cla-%s-gerrit-instances", repo.stage)
	err := repo.scanTable(gerritInstancesTableName, projection, nil, &gerritInstances)
	if err != nil {
		return err
	}
	for _, gi := range gerritInstances {
		metrics.TotalCountMetrics.GerritRepositoriesCount++
		// TODO: how do we check if they're enabled
		metrics.TotalCountMetrics.GerritRepositoriesEnabledCount++
		metrics.ProjectMetrics.processGerritInstance(gi)
	}
	return nil
}

func (repo *repo) cacheUsersByLfUsername() (map[string]*ItemUser, error) {
	usersCache := make(map[string]*ItemUser)
	userTableName := fmt.Sprintf("cla-%s-users", repo.stage)
	log.Println("processing users table")
	projection := expression.NamesList(
		expression.Name("lf_username"),
	)
	var users []*ItemUser
	err := repo.scanTable(userTableName, projection, nil, &users)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		if user.LfUsername != "" {
			usersCache[user.LfUsername] = user
		}
	}
	return usersCache, nil
}

func (repo *repo) processProjectsTable(metrics *Metrics) error {
	projectTableName := fmt.Sprintf("cla-%s-projects", repo.stage)
	log.Println("processing project table")
	projection := expression.NamesList(
		expression.Name("project_id"),
		expression.Name("project_external_id"),
		expression.Name("project_name"),
		expression.Name("project_live"),
	)
	var projects []*ItemProject
	err := repo.scanTable(projectTableName, projection, nil, &projects)
	if err != nil {
		return err
	}
	for _, project := range projects {
		metrics.TotalCountMetrics.ProjectsCount++
		if project.ProjectLive {
			metrics.TotalCountMetrics.ProjectsLiveCount++
		}
		metrics.ProjectMetrics.processProjectItem(project, repo.apiGatewayURL)
	}
	return nil
}

func (repo *repo) processCompaniesTable(metrics *Metrics) error {
	companiesTableName := fmt.Sprintf("cla-%s-companies", repo.stage)
	log.Println("processing companies table")
	projection := expression.NamesList(
		expression.Name("company_id"),
		expression.Name("company_name"),
	)
	var companies []*ItemCompany
	err := repo.scanTable(companiesTableName, projection, nil, &companies)
	if err != nil {
		return err
	}
	for _, company := range companies {
		metrics.CompanyMetrics.processCompanyItem(company)
		metrics.TotalCountMetrics.CompaniesCount++
	}
	return nil
}

func (repo *repo) scanTable(tableName string, projection expression.ProjectionBuilder, filter *expression.ConditionBuilder, output interface{}) error {
	builder := expression.NewBuilder()
	builder = builder.WithProjection(projection)
	if filter != nil {
		builder = builder.WithFilter(*filter)
	}
	expr, err := builder.Build()
	if err != nil {
		log.Warnf("error building expression for %s scan, error: %v", tableName, err)
		return err
	}
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(tableName),
	}
	var resultList []map[string]*dynamodb.AttributeValue
	for {
		results, err := repo.dynamoDBClient.Scan(scanInput) //nolint
		if err != nil {
			log.Warnf("error retrieving %s, error: %v", tableName, err)
			return err
		}
		resultList = append(resultList, results.Items...)
		if len(results.LastEvaluatedKey) != 0 {
			scanInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}
	err = dynamodbattribute.UnmarshalListOfMaps(resultList, &output)
	if err != nil {
		log.Warnf("error unmarshalling %s from database. error: %v", tableName, err)
		return err
	}
	return nil
}

func (repo *repo) calculateMetrics() (*Metrics, error) {
	metrics := newMetrics()
	t := time.Now()
	// build users cache by lf-username
	usersCache, err := repo.cacheUsersByLfUsername()
	if err != nil {
		return nil, err
	}

	log.Debug("Calculating CLA Group metrics...")
	// calculate project count
	// create structure for projectMetric
	// cache project membership info
	err = repo.processProjectsTable(metrics)
	if err != nil {
		return nil, err
	}

	log.Debug("Calculating Org/Company metrics...")
	// calculate companies count
	// create structure for companyMetric
	err = repo.processCompaniesTable(metrics)
	if err != nil {
		return nil, err
	}

	log.Debug("Calculating Signature metrics...")
	// calculate project metrics
	// calculate company metrics
	// calculate total count metrics
	err = repo.processSignaturesTable(metrics, usersCache)
	if err != nil {
		return nil, err
	}

	log.Debug("Calculating CombinedRepository metrics...")
	// calculate github repositories count
	// increment project repositories count
	err = repo.processRepositoriesTable(metrics)
	if err != nil {
		return nil, err
	}

	log.Debug("Calculating Gerrit metrics...")
	// calculate gerrit repositories count
	// increment project repositories count
	err = repo.processGerritInstancesTable(metrics)
	if err != nil {
		return nil, err
	}

	log.Debug("Calculating CLA Manager distribution metrics...")
	metrics.ClaManagersDistribution = calculateClaManagerDistribution(metrics.CompanyMetrics)
	_, metrics.CalculatedAt = utils.CurrentTime()
	log.Println("calculate metrics took time", time.Since(t).String())

	log.Debug("Completed metrics calculations")
	return metrics, nil
}

func (repo *repo) saveMetrics(metrics *Metrics) error {
	t := time.Now()
	err := repo.saveTotalMetrics(metrics.TotalCountMetrics)
	if err != nil {
		return err
	}
	err = repo.saveCompaniesMetrics(metrics.CompanyMetrics)
	if err != nil {
		return err
	}
	err = repo.saveProjectMetrics(metrics.ProjectMetrics)
	if err != nil {
		return err
	}
	err = repo.saveClaManagerDistribution(metrics.ClaManagersDistribution)
	if err != nil {
		return err
	}
	err = repo.saveCompanyProjectMetrics(metrics.CompanyProjectMetrics, metrics.ProjectMetrics.ProjectMetrics, metrics.CompanyMetrics.CompanyMetrics)
	if err != nil {
		return err
	}
	log.Printf("save metrics took :%s \n", time.Since(t).String())
	return nil
}

func (repo *repo) clearOldMetrics(beforeTime time.Time) error {
	t := time.Now()
	filter := expression.Name("created_at").LessThan(expression.Value(utils.TimeToString(beforeTime)))
	type ItemMetric struct {
		ID         string `json:"id"`
		MetricType string `json:"metric_type"`
	}
	projection := expression.NamesList(
		expression.Name("id"),
		expression.Name("metric_type"),
	)
	expr, err := expression.NewBuilder().WithFilter(filter).WithProjection(projection).Build()
	if err != nil {
		log.Warnf("error building expression for metric scan, error: %v", err)
		return err
	}

	// Assemble the query input parameters
	scanInput := &dynamodb.ScanInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		FilterExpression:          expr.Filter(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.metricTableName),
	}

	for {
		results, err := repo.dynamoDBClient.Scan(scanInput)
		if err != nil {
			log.Warnf("error retrieving metrics, error: %v", err)
			return err
		}

		var metrics []*ItemMetric

		err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &metrics)
		if err != nil {
			log.Warnf("error unmarshalling metrics from database. error: %v", err)
			return err
		}

		for _, m := range metrics {
			_, err = repo.dynamoDBClient.DeleteItem(&dynamodb.DeleteItemInput{
				Key: map[string]*dynamodb.AttributeValue{
					"id":          {S: aws.String(m.ID)},
					"metric_type": {S: aws.String(m.MetricType)},
				},
				TableName: aws.String(repo.metricTableName),
			})
			if err != nil {
				log.Error(fmt.Sprintf("error deleting outdated metric with id:%s, metric_type:%s", m.ID, m.MetricType), err)
			}
		}

		if len(results.LastEvaluatedKey) != 0 {
			scanInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}
	log.Printf("clear old metrics took :%s \n", time.Since(t).String())
	return nil
}

func addIDTypeTime(item map[string]*dynamodb.AttributeValue, id string, metricType string) {
	_, ctime := utils.CurrentTime()
	utils.AddStringAttribute(item, "id", id)
	utils.AddStringAttribute(item, "metric_type", metricType)
	utils.AddStringAttribute(item, "created_at", ctime)
}

func (repo *repo) saveTotalMetrics(tm *TotalCountMetrics) error {
	log.Println("saving total count metrics")
	tm.RepositoriesCount = tm.GithubRepositoriesCount + tm.GerritRepositoriesCount
	av, err := dynamodbattribute.MarshalMap(tm)
	if err != nil {
		return err
	}
	addIDTypeTime(av, IDTotalCount, MetricTypeTotalCount)
	_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(repo.metricTableName),
	})
	if err != nil {
		log.Error("cannot put total_metrics in dynamodb", err)
		return err
	}
	return nil
}

func (repo *repo) saveCompaniesMetrics(companyMetrics *CompanyMetrics) error {
	t := time.Now()
	log.Println("saving company_metrics")
	for id, cm := range companyMetrics.CompanyMetrics {
		av, err := dynamodbattribute.MarshalMap(cm)
		if err != nil {
			return err
		}
		addIDTypeTime(av, id, MetricTypeCompany)
		_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
			Item:      av,
			TableName: aws.String(repo.metricTableName),
		})
		if err != nil {
			log.Printf("cannot put company_metric in dynamodb, metric = %v, error = %s\n", cm, err.Error())
			return err
		}
	}
	log.Printf("saving company_metrics took :%s \n", time.Since(t).String())
	return nil
}

func (repo *repo) saveClaManagerDistribution(cmd *ClaManagersDistribution) error {
	log.Println("saving cla_managers_distribution")
	av, err := dynamodbattribute.MarshalMap(cmd)
	if err != nil {
		return err
	}
	addIDTypeTime(av, IDClaManagerDistribution, MetricTypeClaManagerDistribution)
	_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(repo.metricTableName),
	})
	if err != nil {
		log.Printf("cannot put cla_managers_distribution in dynamodb, metric = %v, error = %s\n", cmd, err.Error())
		return err
	}
	return nil
}

func (repo *repo) saveProjectMetrics(projectMetrics *ProjectMetrics) error {
	t := time.Now()
	log.Println("saving project_metrics")
	for id, cm := range projectMetrics.ProjectMetrics {
		av, err := dynamodbattribute.MarshalMap(cm)
		if err != nil {
			return err
		}
		addIDTypeTime(av, id, MetricTypeProject)
		_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
			Item:      av,
			TableName: aws.String(repo.metricTableName),
		})
		if err != nil {
			log.Printf("cannot put project_metric in dynamodb, metric = %v, error = %s\n", cm, err.Error())
			return err
		}
	}
	log.Printf("saving project_metrics took :%s \n", time.Since(t).String())
	return nil
}

type claGroup struct {
	claGroupID      string
	projectSFIDList []string
	foundationSFID  string
}

func (repo *repo) getClaGroupProjectsMapping() (map[string]*claGroup, error) {
	r := make(map[string]*claGroup)
	cgpList, err := repo.projectsClaGroupsRepo.GetProjectsIdsForAllFoundation(context.Background())
	if err != nil {
		return r, err
	}
	for _, cgp := range cgpList {
		cg, ok := r[cgp.ClaGroupID]
		if !ok {
			cg = &claGroup{
				claGroupID:      cgp.ClaGroupID,
				projectSFIDList: []string{cgp.ProjectSFID},
				foundationSFID:  cgp.FoundationSFID,
			}
			r[cgp.ClaGroupID] = cg
		} else {
			cg.projectSFIDList = append(cg.projectSFIDList, cgp.ProjectSFID)
		}
	}
	return r, nil
}

func (repo *repo) saveCompanyProjectMetrics(in *CompanyProjectMetrics, pmm map[string]*ProjectMetric, cmm map[string]*CompanyMetric) error {
	t := time.Now()
	log.Println("saving company_project_metrics")

	claGroupMapping, err := repo.getClaGroupProjectsMapping()
	if err != nil {
		return err
	}

	filterProjectMap := repo.projectHelperMap(in, claGroupMapping)

	for id, cpm := range in.CompanyProjectMetrics {
		pm, ok := pmm[cpm.ProjectID]
		if !ok {
			log.Warnf("saveCompanyProjectMetrics error = project not found with id. [%s]", cpm.ProjectID)
			continue
		}
		cm, ok := cmm[cpm.CompanyID]
		if !ok {
			log.Warnf("saveCompanyProjectMetrics error = company not found with id. [%s]", cpm.CompanyID)
			continue
		}
		claGroupMap, ok := claGroupMapping[cpm.ProjectID]
		if !ok {
			log.Warnf("saveCompanyProjectMetrics error = cla group not present in cla-group project mapping. [%s]", cpm.ProjectID)
			continue
		}
		if len(claGroupMap.projectSFIDList) == 1 {
			cpm.ProjectSFID = claGroupMap.projectSFIDList[0]
		} else {
			cpm.ProjectSFID = claGroupMap.foundationSFID
		}

		projectName, ok := filterProjectMap[cpm.ProjectSFID]
		if !ok {
			log.Warnf("saveCompanyProjectMetrics error = unable to get project details from project-service. %s", cpm.ProjectSFID)
			continue
		}
		cpm.ProjectName = projectName

		cpm.CompanyName = cm.CompanyName
		cpm.ClaGroupName = pm.ProjectName
		av, err := dynamodbattribute.MarshalMap(cpm)
		if err != nil {
			return err
		}

		addIDTypeTime(av, id, MetricTypeCompanyProject)
		_, err = repo.dynamoDBClient.PutItem(&dynamodb.PutItemInput{
			Item:      av,
			TableName: aws.String(repo.metricTableName),
		})
		if err != nil {
			log.Printf("cannot put company_project_metric in dynamodb, metric = %v, error = %s\n", cpm, err.Error())

			return err
		}
	}
	log.Printf("saving company_project_metrics took :%s \n", time.Since(t).String())
	return nil
}

func (repo *repo) projectHelperMap(in *CompanyProjectMetrics, claGroupMapping map[string]*claGroup) map[string]string {
	projectIDArray := map[string]bool{}
	filterProjectMap := map[string]string{}

	psc := project_service.GetClient()

	for _, cpm := range in.CompanyProjectMetrics {
		claGroupMap, ok := claGroupMapping[cpm.ProjectID]
		if !ok {
			continue
		}
		if len(claGroupMap.projectSFIDList) == 1 {
			cpm.ProjectSFID = claGroupMap.projectSFIDList[0]
		} else {
			cpm.ProjectSFID = claGroupMap.foundationSFID
		}
		if _, ok := projectIDArray[cpm.ProjectSFID]; !ok {
			projectIDArray[cpm.ProjectSFID] = true
		}
	}

	log.Printf("length of projectIDArray %d", len(projectIDArray))

	for projectSFID := range projectIDArray {
		projectData, err := repo.projectsClaGroupsRepo.GetClaGroupIDForProject(context.Background(), projectSFID)
		if err != nil {
			log.Warnf("projectHelperMap/GetClaGroupIDForProject error = unable to get project details from easycla. %s", projectSFID)
			continue
		}

		if projectData == nil {
			projectDetails, err := psc.GetProject(projectSFID)
			if err != nil {
				log.Warnf("projectHelperMap/GetProject error = unable to get project details from project-service. %s", projectSFID)
				continue
			}
			filterProjectMap[projectSFID] = projectDetails.Name
		}
		filterProjectMap[projectData.ProjectSFID] = projectData.ProjectName

	}
	log.Printf("Length of filterProjectMap %d \n", len(filterProjectMap))
	return filterProjectMap
}

func (repo *repo) CalculateAndSaveMetrics() error {
	timeBeforeStartingMetricsCalculation := time.Now()
	m, err := repo.calculateMetrics()
	if err != nil {
		return err
	}
	err = repo.saveMetrics(m)
	if err != nil {
		return err
	}
	err = repo.clearOldMetrics(timeBeforeStartingMetricsCalculation)
	if err != nil {
		return err
	}
	return nil
}

func (repo *repo) GetClaManagerDistribution() (*ClaManagersDistribution, error) {
	var out ClaManagersDistribution
	err := repo.getMetricByID(IDClaManagerDistribution, MetricTypeClaManagerDistribution, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (repo *repo) GetTotalCountMetrics() (*TotalCountMetrics, error) {
	var out TotalCountMetrics
	err := repo.getMetricByID(IDTotalCount, MetricTypeTotalCount, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (repo *repo) GetCompanyMetrics() ([]*CompanyMetric, error) {

	keyCondition := expression.Key("metric_type").Equal(expression.Value(MetricTypeCompany))

	expr, err := expression.NewBuilder().WithKeyCondition(keyCondition).Build()
	if err != nil {
		log.Warnf("error building expression for company metric scan, error: %v", err)
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		TableName:                 aws.String(repo.metricTableName),
	}

	var companyMetrics []*CompanyMetric
	for {
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.Warnf("error retrieving company metrics, error: %v", errQuery)
			return nil, errQuery
		}

		var companyMetricsTmp []*CompanyMetric

		err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &companyMetricsTmp)
		if err != nil {
			log.Warnf("error unmarshalling company metrics from database. error: %v", err)
			return nil, err
		}
		companyMetrics = append(companyMetrics, companyMetricsTmp...)

		if len(results.LastEvaluatedKey) != 0 {
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}
	return companyMetrics, nil
}

func (repo *repo) GetProjectMetrics(pageSize int64, nextKey string) ([]*ProjectMetric, string, error) {

	keyCondition := expression.Key("metric_type").Equal(expression.Value(MetricTypeProject))

	expr, err := expression.NewBuilder().WithKeyCondition(keyCondition).Build()
	if err != nil {
		log.Warnf("error building expression for company metric scan, error: %v", err)
		return nil, "", err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		TableName:                 aws.String(repo.metricTableName),
		Limit:                     aws.Int64(pageSize),
	}

	if nextKey != "" {
		queryInput.ExclusiveStartKey = map[string]*dynamodb.AttributeValue{
			"id":          {S: aws.String(nextKey)},
			"metric_type": {S: aws.String(MetricTypeProject)},
		}
	}

	var projectMetrics []*ProjectMetric
	for {
		results, err := repo.dynamoDBClient.Query(queryInput)
		if err != nil {
			log.Warnf("error retrieving project metrics, error: %v", err)
			return nil, "", err
		}

		var projectMetricsTmp []*ProjectMetric

		err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &projectMetricsTmp)
		if err != nil {
			log.Warnf("error unmarshalling project metrics from database. error: %v", err)
			return nil, "", err
		}
		projectMetrics = append(projectMetrics, projectMetricsTmp...)
		if len(results.LastEvaluatedKey) != 0 {
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
			nextKey = *results.LastEvaluatedKey["id"].S
		} else {
			nextKey = ""
			break
		}
		if int64(len(projectMetrics)) >= pageSize {
			break
		}
	}
	return projectMetrics, nextKey, nil
}

func (repo *repo) GetCompanyMetric(companyID string) (*CompanyMetric, error) {
	var out CompanyMetric
	err := repo.getMetricByID(companyID, MetricTypeCompany, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (repo *repo) GetProjectMetric(projectID string) (*ProjectMetric, error) {
	var out ProjectMetric
	err := repo.getMetricByID(projectID, MetricTypeProject, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (repo *repo) GetProjectMetricBySalesForceID(salesforceID string) ([]*ProjectMetric, error) {
	var out []*ProjectMetric
	err := repo.getMetricBySalesforceID(salesforceID, MetricTypeProject, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (repo *repo) getMetricByID(id string, metricType string, out interface{}) error {
	result, err := repo.dynamoDBClient.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(repo.metricTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(id),
			},
			"metric_type": {
				S: aws.String(metricType),
			},
		},
	})
	if err != nil {
		return err
	}
	if len(result.Item) == 0 {
		return ErrMetricNotFound
	}
	err = dynamodbattribute.UnmarshalMap(result.Item, out)
	if err != nil {
		return err
	}
	return nil
}

func (repo *repo) getMetricBySalesforceID(salesforceID string, metricType string, out interface{}) error {
	var condition expression.KeyConditionBuilder
	builder := expression.NewBuilder()

	condition = expression.Key("metric_type").Equal(expression.Value(metricType)).
		And(expression.Key("salesforce_id").Equal(expression.Value(salesforceID)))

	builder = builder.WithKeyCondition(condition)
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		return err
	}
	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.metricTableName),
		IndexName:                 aws.String(IndexMetricTypeSalesforceID),
	}

	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.Warnf("error retrieving metrics using salesforce_id. error = %s", err.Error())
		return err
	}
	if len(results.Items) == 0 {
		return ErrMetricNotFound
	}
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, out)
	if err != nil {
		return err
	}
	return nil
}

func (repo *repo) ListCompanyProjectMetrics(companyID string) ([]*CompanyProjectMetric, error) {
	var condition expression.KeyConditionBuilder
	builder := expression.NewBuilder()
	prefix := companyID + "#"
	condition = expression.Key("metric_type").Equal(expression.Value(MetricTypeCompanyProject)).
		And(expression.Key("id").BeginsWith(prefix))

	builder = builder.WithKeyCondition(condition)
	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.metricTableName),
	}

	results, err := repo.dynamoDBClient.Query(queryInput)
	if err != nil {
		log.Warnf("error retrieving company_project metrics. error = %s", err.Error())
		return nil, err
	}
	out := make([]*CompanyProjectMetric, 0)
	err = dynamodbattribute.UnmarshalListOfMaps(results.Items, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
