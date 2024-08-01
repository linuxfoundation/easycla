// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gerrits

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	// "github.com/LF-Engineering/lfx-kit/auth"

	"github.com/go-openapi/strfmt"

	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	// v2Models "github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// Service handles gerrit Repository service
type Service interface {
	AddGerrit(ctx context.Context, claGroupID string, projectSFID string, input *models.AddGerritInput, claGroupModel *models.ClaGroup) (*models.Gerrit, error)
	GetGerrit(ctx context.Context, gerritID string) (*models.Gerrit, error)
	GetGerritsByProjectSFID(ctx context.Context, projectSFID string) (*models.GerritList, error)
	GetClaGroupGerrits(ctx context.Context, claGroupID string) (*models.GerritList, error)
	GetGerritRepos(ctx context.Context, gerritName string) (*models.GerritRepoList, error)
	DeleteClaGroupGerrits(ctx context.Context, claGroupID string) (int, error)
	DeleteGerrit(ctx context.Context, gerritID string) error
}

type service struct {
	repo Repository
}

// NewService creates a new gerrit service
func NewService(repo Repository) Service {
	return service{
		repo: repo,
	}
}

func (s service) AddGerrit(ctx context.Context, claGroupID string, projectSFID string, params *models.AddGerritInput, claGroupModel *models.ClaGroup) (*models.Gerrit, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.service.AddGerrit",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"projectSFID":    projectSFID,
	}

	log.WithFields(f).Debugf("cla groupID %s", claGroupID)
	log.WithFields(f).Debugf("project Model %+v", claGroupModel)

	if params.GerritName == nil {
		return nil, errors.New("gerrit_name required")
	}

	gerritObject, err := s.repo.ExistsByName(ctx, *params.GerritName)
	if err != nil {
		message := fmt.Sprintf("unable to get gerrit by name : %s", *params.GerritName)
		log.WithFields(f).WithError(err).Warnf(message)
	}

	if len(gerritObject) > 0 {
		return nil, errors.New("gerrit_name already present in the system")
	}

	if params.GerritURL == nil {
		return nil, errors.New("gerrit_url required")
	}

	input := &models.Gerrit{
		GerritName:  utils.StringValue(params.GerritName),
		GerritURL:   strfmt.URI(*params.GerritURL),
		ProjectID:   claGroupID,
		ProjectSFID: projectSFID,
		Version:     params.Version,
	}
	return s.repo.AddGerrit(ctx, input)
}

func (s service) GetGerrit(ctx context.Context, gerritID string) (*models.Gerrit, error) {
	return s.repo.GetGerrit(ctx, gerritID)
}

// GetGerritsByProjectSFID returns a list of gerrit instances based on the projectSFID
func (s service) GetGerritsByProjectSFID(ctx context.Context, projectSFID string) (*models.GerritList, error) {
	return s.repo.GetGerritsByProjectSFID(ctx, projectSFID)
}

func (s service) GetClaGroupGerrits(ctx context.Context, claGroupID string) (*models.GerritList, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.service.GetClaGroupGerrits",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
	}
	responseModel, err := s.repo.GetClaGroupGerrits(ctx, claGroupID)
	if err != nil {
		log.WithFields(f).Warnf("problem getting CLA Group gerrits, error: %+v", err)
		return nil, err
	}

	log.WithFields(f).Debugf("discovered %d gerrits", len(responseModel.List))
	// Add the repo list to the response model
	for _, gerrit := range responseModel.List {
		log.WithFields(f).Debugf("Processing gerrit URL: %s", gerrit.GerritURL)

		var gerritHost = gerrit.GerritURL.String()
		gerritHost, err = extractGerritHost(gerritHost, f)
		if err != nil {
			return nil, err
		}

		log.WithFields(f).Debugf("fetching gerrit repos from host: %s", gerritHost)
		gerritRepoList, getRepoErr := s.GetGerritRepos(ctx, gerritHost)
		if getRepoErr != nil {
			log.WithFields(f).Warnf("problem fetching gerrit repos from host: %s, error: %+v", gerritHost, err)
			log.Error("skipping", getRepoErr)
			continue
			//return nil, getRepoErr
		}

		// Set the connected flag - for now, we just set this value to true
		for _, repo := range gerritRepoList.Repos {
			repo.Connected = true
		}

		gerrit.GerritRepoList = gerritRepoList
	}

	return responseModel, err
}

func extractGerritHost(gerritHost string, f logrus.Fields) (string, error) {
	if strings.HasPrefix(gerritHost, "http") {
		log.WithFields(f).Debugf("extracting gerrit host from URL: %s", gerritHost)
		u, urlErr := url.Parse(gerritHost)
		if urlErr != nil {
			log.WithFields(f).Warnf("problem converting gerrit URL: %s, error: %+v", gerritHost, urlErr)
			return "", urlErr
		}
		gerritHost = u.Host
		log.WithFields(f).Debugf("extracted gerrit host is: %s", gerritHost)
	}
	return gerritHost, nil
}

func (s service) GetGerritRepos(ctx context.Context, gerritHost string) (*models.GerritRepoList, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.service.GetGerritRepos",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"gerritName":     gerritHost,
	}

	gerritRepos, err := listGerritRepos(ctx, gerritHost)
	if err != nil {
		log.WithFields(f).Warnf("problem querying gerrit host, error: %+v", err)
		return nil, err
	}

	gerritConfig, err := getGerritConfig(ctx, gerritHost)
	if err != nil {
		log.WithFields(f).Warnf("problem querying gerrit config, error: %+v", err)
		return nil, err
	}

	return convertModel(gerritRepos, gerritConfig), nil
}

func (s service) DeleteClaGroupGerrits(ctx context.Context, claGroupID string) (int, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.service.DeleteClaGroupGerrits",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
	}

	gerrits, err := s.repo.GetClaGroupGerrits(ctx, claGroupID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem fetching gerrits for CLA Group: %s", claGroupID)
		return 0, err
	}
	if len(gerrits.List) > 0 {
		log.WithFields(f).Debugf(fmt.Sprintf("Deleting gerrits for cla-group :%s ", claGroupID))
		for _, gerrit := range gerrits.List {
			err = s.repo.DeleteGerrit(ctx, gerrit.GerritID.String())
			if err != nil {
				return 0, err
			}
		}
	}

	return len(gerrits.List), nil
}

func (s service) DeleteGerrit(ctx context.Context, gerritID string) error {
	return s.repo.DeleteGerrit(ctx, gerritID)
}

// convertModel is a helper function to create a GerritRepoList response model
func convertModel(responseModel map[string]GerritRepoInfo, serverInfo *ServerInfo) *models.GerritRepoList {
	var gerritRepos []*models.GerritRepo
	for name, repo := range responseModel {

		var weblinks []*models.GerritRepoWebLinksItems0
		for _, weblink := range repo.WebLinks {
			weblinks = append(weblinks, &models.GerritRepoWebLinksItems0{
				Name: weblink.Name,
				URL:  strfmt.URI(weblink.URL),
			})
		}

		claEnabled := false
		if serverInfo != nil && serverInfo.Auth.UseContributorAgreements {
			claEnabled = true
		}

		gerritRepos = append(gerritRepos, &models.GerritRepo{
			ID:                    repo.ID,
			Name:                  name,
			Description:           repo.Description,
			State:                 repo.State,
			ClaEnabled:            claEnabled,
			ContributorAgreements: buildContributorAgreementDetails(serverInfo),
			WebLinks:              weblinks,
		})
	}

	return &models.GerritRepoList{
		Repos: gerritRepos,
	}
}

// buildContributorAgreementDetails helper function to extract and convert the gerrit server info contributor agreement information into a response data model
func buildContributorAgreementDetails(serverInfo *ServerInfo) []*models.GerritRepoContributorAgreementsItems0 {
	var response []*models.GerritRepoContributorAgreementsItems0

	for _, agreement := range serverInfo.Auth.ContributorAgreements {
		response = append(response, &models.GerritRepoContributorAgreementsItems0{
			Name:        agreement.Name,
			Description: agreement.Description,
			URL:         strfmt.URI(agreement.URL),
		})
	}

	return response
}

// listGerritRepos returns a list of gerrit repositories for the given gerrit host
func listGerritRepos(ctx context.Context, gerritHost string) (map[string]GerritRepoInfo, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.listGerritRepos",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"gerritHost":     gerritHost,
	}
	client := resty.New()

	base := "https://" + gerritHost

	gerritAPIPath, gerritAPIPathErr := getGerritAPIPath(ctx, gerritHost)
	if gerritAPIPathErr != nil {
		return nil, gerritAPIPathErr
	}

	if gerritAPIPath != "" {
		base = fmt.Sprintf("https://%s/%s", gerritHost, gerritAPIPath)
	}

	resp, err := client.R().
		EnableTrace().
		Get(fmt.Sprintf("%s/projects/?d&pp=0", base))
	if err != nil {
		log.WithFields(f).Warnf("problem querying gerrit host: %s, error: %+v", gerritHost, err)
		return nil, err
	}

	if resp.IsError() {
		msg := fmt.Sprintf("non-success response from list gerrit host repos for gerrit %s, error code: %s", gerritHost, resp.Status())
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	var result map[string]GerritRepoInfo
	// Need to strip off the leading "magic prefix line" from the response payload, which is: )]}'
	// See: https://gerrit.linuxfoundation.org/infra/Documentation/rest-api.html#output
	err = json.Unmarshal(resp.Body()[4:], &result)
	if err != nil {
		log.WithFields(f).Warnf("problem unmarshalling response for gerrit host: %s, error: %+v", gerritHost, err)
		return nil, err
	}

	return result, nil
}

// getGerritConfig returns the gerrit configuration for the specified host
func getGerritConfig(ctx context.Context, gerritHost string) (*ServerInfo, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.getGerritConfig",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"gerritHost":     gerritHost,
	}
	client := resty.New()

	base := "https://" + gerritHost

	gerritAPIPath, gerritAPIPathErr := getGerritAPIPath(ctx, gerritHost)
	if gerritAPIPathErr != nil {
		return nil, gerritAPIPathErr
	}

	if gerritAPIPath != "" {
		base = fmt.Sprintf("https://%s/%s", gerritHost, gerritAPIPath)
	}

	resp, err := client.R().
		EnableTrace().
		Get(fmt.Sprintf("%s/config/server/info", base))
	if err != nil {
		log.WithFields(f).Warnf("problem querying gerrit config, error: %+v", err)
		return nil, err
	}

	if resp.IsError() {
		msg := fmt.Sprintf("non-success response from list gerrit host config query, error code: %s", resp.Status())
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	var result ServerInfo
	// Need to strip off the leading "magic prefix line" from the response payload, which is: )]}'
	// See: https://gerrit.linuxfoundation.org/infra/Documentation/rest-api.html#output
	err = json.Unmarshal(resp.Body()[4:], &result)
	if err != nil {
		log.WithFields(f).Warnf("problem unmarshalling response for gerrit host: %s, error: %+v", gerritHost, err)
		return nil, err
	}

	return &result, nil
}

// getGerritAPIPath returns the path to the API based on the gerrit host
func getGerritAPIPath(ctx context.Context, gerritHost string) (string, error) {
	f := logrus.Fields{
		"functionName":   "v1.gerrits.getGerritAPIPath",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"gerritHost":     gerritHost,
	}
	switch gerritHost {
	case "gerrit.linuxfoundation.org":
		return "infra", nil
	case "mockapi.gerrit.dev.itx.linuxfoundation.org":
		return "", nil
	case "gerrit.onap.org":
		return "r", nil
	case "gerrit.o-ran-sc.org":
		return "r", nil
	case "gerrit.tungsten.io":
		return "r", nil
	case "gerrit.opnfv.org":
		return "gerrit", nil
	default:
		msg := fmt.Sprintf("unsupport gerrit host: %s", gerritHost)
		log.WithFields(f).Warnf(msg)
		return "", errors.New(msg)
	}
}
