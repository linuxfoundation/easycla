// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package metrics

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	project_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
)

// constants
const (
	// FoundationType the SF foundation type string - previously was "Foundation", now "Project Group"
	FoundationType = "Project Group"
)

// Service interface defines function of Metrics service
type Service interface {
	GetCLAManagerDistribution() (*models.ClaManagerDistribution, error)
	GetTotalCountMetrics() (*models.TotalCountMetrics, error)
	GetCompanyMetric(companyID string) (*models.CompanyMetric, error)
	GetProjectMetric(projectID string, idType string) (*models.SfProjectMetric, error)
	GetTopCompanies() (*models.TopCompanies, error)
	GetTopProjects() (*models.TopProjects, error)
	ListProjectMetrics(paramPageSize *int64, paramNextKey *string) (*models.ListProjectMetric, error)
	ListCompanyProjectMetrics(ctx context.Context, companyID string, projectSFID string) (*models.CompanyProjectMetrics, error)
}

type service struct {
	metricsRepo           Repository
	projectsClaGroupsRepo projects_cla_groups.Repository
}

// NewService creates new instance of metrics service
func NewService(metricsRepo Repository, pcgRepo projects_cla_groups.Repository) Service {
	return &service{
		metricsRepo:           metricsRepo,
		projectsClaGroupsRepo: pcgRepo,
	}
}

func (s *service) GetCLAManagerDistribution() (*models.ClaManagerDistribution, error) {
	cmd, err := s.metricsRepo.GetClaManagerDistribution()
	if err != nil {
		return nil, err
	}
	return &models.ClaManagerDistribution{
		FourOrMoreClaManagers: cmd.FourOrMoreClaManager,
		OneClaManager:         cmd.OneClaManager,
		ThreeClaManagers:      cmd.ThreeClaManager,
		TwoClaManagers:        cmd.TwoClaManager,
		CreatedAt:             cmd.CreatedAt,
	}, nil
}

func (s *service) GetTotalCountMetrics() (*models.TotalCountMetrics, error) {
	tcm, err := s.metricsRepo.GetTotalCountMetrics()
	if err != nil {
		return nil, err
	}
	return tcm.toModel(), nil
}

func (s *service) GetCompanyMetric(companyID string) (*models.CompanyMetric, error) {
	cm, err := s.metricsRepo.GetCompanyMetric(companyID)
	if err != nil {
		return nil, err
	}
	return cm.toModel(), nil
}

func (s *service) GetProjectMetric(projectID string, idType string) (*models.SfProjectMetric, error) {
	sfpm := &models.SfProjectMetric{}
	switch idType {
	case "internal":
		pm, err := s.metricsRepo.GetProjectMetric(projectID)
		if err != nil {
			return nil, err
		}
		sfpm.ProjectExternalID = pm.ExternalProjectID
		sfpm.List = append(sfpm.List, pm.toModel())
	case "salesforce":
		pmList, err := s.metricsRepo.GetProjectMetricBySalesForceID(projectID)
		if err != nil {
			return nil, err
		}
		sfpm.ProjectExternalID = projectID
		for _, pm := range pmList {
			sfpm.List = append(sfpm.List, pm.toModel())
		}
	default:
		return nil, errors.New("invalid idType")
	}
	return sfpm, nil
}

func average(numerator, denominator int64) int64 {
	return int64(math.Round(float64(numerator) / float64(denominator)))
}

func (s *service) GetTopCompanies() (*models.TopCompanies, error) {
	var averageClaManagers, averageCorporateContributors, averageProjects int64
	returnCount := 5
	cmetrics, err := s.metricsRepo.GetCompanyMetrics()
	if err != nil {
		return nil, err
	}
	if len(cmetrics) < returnCount {
		returnCount = len(cmetrics)
	}
	cmByCorporateContributors := make([]*CompanyMetric, len(cmetrics))
	cmByClaManagers := make([]*CompanyMetric, len(cmetrics))
	cmByProjectCount := make([]*CompanyMetric, len(cmetrics))
	copy(cmByCorporateContributors, cmetrics)
	copy(cmByClaManagers, cmetrics)
	copy(cmByProjectCount, cmetrics)

	var wg sync.WaitGroup
	wg.Add(4)
	go func() {
		defer wg.Done()
		tm, err := s.metricsRepo.GetTotalCountMetrics()
		if err != nil {
			log.Warnf("unable to get total count metrics. error = %s", err.Error())
			return
		}
		averageClaManagers = average(tm.ClaManagersCount, tm.CompaniesCount)
		averageCorporateContributors = average(tm.CorporateContributorsCount, tm.CompaniesCount)
		averageProjects = average(tm.CompaniesProjectContributionCount, tm.CompaniesCount)
	}()
	go func() {
		defer wg.Done()
		sort.Slice(cmByProjectCount, func(i, j int) bool {
			if cmByProjectCount[i].ProjectCount == cmByProjectCount[j].ProjectCount {
				return strings.ToLower(cmByProjectCount[i].CompanyName) < strings.ToLower(cmByProjectCount[j].CompanyName)
			}
			return cmByProjectCount[i].ProjectCount > cmByProjectCount[j].ProjectCount
		})
	}()

	go func() {
		defer wg.Done()
		sort.Slice(cmByCorporateContributors, func(i, j int) bool {
			if cmByCorporateContributors[i].CorporateContributorsCount == cmByCorporateContributors[j].CorporateContributorsCount {
				return strings.ToLower(cmByCorporateContributors[i].CompanyName) < strings.ToLower(cmByCorporateContributors[j].CompanyName)
			}
			return cmByCorporateContributors[i].CorporateContributorsCount > cmByCorporateContributors[j].CorporateContributorsCount
		})
	}()

	go func() {
		defer wg.Done()
		sort.Slice(cmByClaManagers, func(i, j int) bool {
			if cmByClaManagers[i].ClaManagersCount == cmByClaManagers[j].ClaManagersCount {
				return strings.ToLower(cmByClaManagers[i].CompanyName) < strings.ToLower(cmByClaManagers[j].CompanyName)
			}
			return cmByClaManagers[i].ClaManagersCount > cmByClaManagers[j].ClaManagersCount
		})
	}()
	wg.Wait()
	return &models.TopCompanies{
		AverageCompanyClaManagers:           averageClaManagers,
		AverageCompanyCorporateContributors: averageCorporateContributors,
		AverageCompanyProjectCount:          averageProjects,
		TopCompaniesByClaManagers:           companiesToModel(cmByClaManagers[:returnCount]),
		TopCompaniesByCorporateContributors: companiesToModel(cmByCorporateContributors[:returnCount]),
		TopCompaniesByProjectCount:          companiesToModel(cmByProjectCount[:returnCount]),
	}, nil
}

func descSortCompare(left, right int64, leftName, rightName string) bool {
	if left == right {
		return leftName < rightName
	}
	return left > right
}

func (s *service) GetTopProjects() (*models.TopProjects, error) {
	returnCount := 5
	var pageSize int64 = 100000
	var pmetrics []*ProjectMetric
	var nextKey string
	for ok := true; ok; ok = nextKey != "" {
		var result []*ProjectMetric
		var err error
		result, nextKey, err = s.metricsRepo.GetProjectMetrics(pageSize, nextKey)
		if err != nil {
			return nil, err
		}
		pmetrics = append(pmetrics, result...)
	}
	if len(pmetrics) < returnCount {
		returnCount = len(pmetrics)
	}
	pmIcla := make([]*ProjectMetric, len(pmetrics))
	pmCcla := make([]*ProjectMetric, len(pmetrics))
	pmIclaAndCcla := make([]*ProjectMetric, len(pmetrics))

	copy(pmIcla, pmetrics)
	copy(pmCcla, pmetrics)
	copy(pmIclaAndCcla, pmetrics)

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		sort.Slice(pmIcla, func(i, j int) bool {
			return descSortCompare(
				pmIcla[i].IndividualContributorsCount,
				pmIcla[j].IndividualContributorsCount,
				pmIcla[i].ProjectName,
				pmIcla[j].ProjectName,
			)
		})
	}()

	go func() {
		defer wg.Done()
		sort.Slice(pmCcla, func(i, j int) bool {
			return descSortCompare(
				pmCcla[i].CompaniesCount,
				pmCcla[j].CompaniesCount,
				pmCcla[i].ProjectName,
				pmCcla[j].ProjectName,
			)
		})
	}()

	go func() {
		defer wg.Done()
		sort.Slice(pmIclaAndCcla, func(i, j int) bool {
			return descSortCompare(
				pmIclaAndCcla[i].IndividualContributorsCount+pmIclaAndCcla[i].CompaniesCount,
				pmIclaAndCcla[j].IndividualContributorsCount+pmIclaAndCcla[j].CompaniesCount,
				pmIclaAndCcla[i].ProjectName,
				pmIclaAndCcla[j].ProjectName,
			)
		})
	}()
	wg.Wait()
	return &models.TopProjects{
		TopProjectsByIcla:        projectsToModel(pmIcla[:returnCount]),
		TopProjectsByCcla:        projectsToModel(pmCcla[:returnCount]),
		TopProjectsByIclaAndCcla: projectsToModel(pmIclaAndCcla[:returnCount]),
	}, nil
}

func (s *service) ListProjectMetrics(paramPageSize *int64, paramNextKey *string) (*models.ListProjectMetric, error) {
	var out models.ListProjectMetric
	var pageSize int64 = 100
	var nextKey string
	if paramPageSize != nil {
		pageSize = *paramPageSize
	}
	if paramNextKey != nil {
		nextKey = *paramNextKey
	}
	list, nextKey, err := s.metricsRepo.GetProjectMetrics(pageSize, nextKey)
	if err != nil {
		return nil, err
	}
	sfProjectMetrics := make(map[string]*models.SfProjectMetric)
	for _, pm := range list {
		sfpm, ok := sfProjectMetrics[pm.ExternalProjectID]
		if !ok {
			sfpm = &models.SfProjectMetric{
				ProjectExternalID: pm.ExternalProjectID,
			}
			sfProjectMetrics[pm.ExternalProjectID] = sfpm
			out.List = append(out.List, sfpm)
		}
		sfpm.List = append(sfpm.List, pm.toModel())
	}
	out.NextKey = nextKey
	return &out, nil
}

func (s *service) ListCompanyProjectMetrics(ctx context.Context, companyID string, projectSFID string) (*models.CompanyProjectMetrics, error) {
	psc := project_service.GetClient()
	claGroupList := utils.NewStringSet()
	project, err := psc.GetProject(projectSFID)
	if err != nil {
		return nil, err
	}
	if project.ProjectType == FoundationType {
		cgmList, cgerr := s.projectsClaGroupsRepo.GetProjectsIdsForFoundation(ctx, projectSFID)
		if cgerr != nil {
			return nil, err
		}
		for _, cgm := range cgmList {
			claGroupList.Add(cgm.ClaGroupID)
		}
	} else {
		cgm, cgerr := s.projectsClaGroupsRepo.GetClaGroupIDForProject(ctx, projectSFID)
		if cgerr != nil {
			return nil, err
		}
		claGroupList.Add(cgm.ClaGroupID)
	}

	list, err := s.metricsRepo.ListCompanyProjectMetrics(companyID)
	if err != nil {
		return nil, err
	}
	out := &models.CompanyProjectMetrics{List: make([]*models.CompanyProjectMetric, 0)}
	for _, cpm := range list {
		if claGroupList.Include(cpm.ProjectID) {
			out.List = append(out.List, cpm.toModel())
		}
	}
	sort.Slice(out.List, func(i, j int) bool {
		return out.List[i].ProjectName < out.List[j].ProjectName
	})
	return out, nil
}
