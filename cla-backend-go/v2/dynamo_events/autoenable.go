// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	service2 "github.com/communitybridge/easycla/cla-backend-go/project/service"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/github_organizations"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/repositories"
	"github.com/go-openapi/swag"
	"github.com/google/go-github/v37/github"
	"github.com/sirupsen/logrus"
)

var (
	// ErrAutoEnabledOff indicates the flag is disabled on github org
	ErrAutoEnabledOff = errors.New("autoEnabled is off")
	// ErrCantDetermineAutoEnableClaGroup indicates the cla group can't be determined for github org
	ErrCantDetermineAutoEnableClaGroup = errors.New("can't determine autoEnable cla-group")
)

// AutoEnableService holds logic about handling autoEnabled field for github Org and Repos
type AutoEnableService interface {
	CreateAutoEnabledRepository(repo *github.Repository) (*models.GithubRepository, error)
	AutoEnabledForGithubOrg(f logrus.Fields, gitHubOrg github_organizations.GithubOrganization, notify bool) error
	NotifyCLAManagerForRepos(claGroupID string, repos []*models.GithubRepository) error
}

// NewAutoEnableService creates a new AutoEnableService
func NewAutoEnableService(repositoryService repositories.Service,
	githubRepo repositories.RepositoryInterface,
	githubOrgRepo github_organizations.RepositoryInterface,
	claRepository projects_cla_groups.Repository,
	claService service2.Service,
) AutoEnableService {
	return &autoEnableServiceProvider{
		repositoryService: repositoryService,
		gitV1Repository:   githubRepo,
		githubOrgRepo:     githubOrgRepo,
		claRepository:     claRepository,
		claService:        claService,
	}
}

// autoEnableServiceProvider is an abstraction helping with managing autoEnabled flag for GitHub Organization
// having it separated in its own struct makes testing easier.
type autoEnableServiceProvider struct {
	repositoryService repositories.Service
	gitV1Repository   repositories.RepositoryInterface
	githubOrgRepo     github_organizations.RepositoryInterface
	claRepository     projects_cla_groups.Repository
	claService        service2.Service
}

func (a *autoEnableServiceProvider) CreateAutoEnabledRepository(repo *github.Repository) (*models.GithubRepository, error) {
	ctx := utils.NewContext()
	repositoryFullName := *repo.FullName
	repositoryExternalID := strconv.FormatInt(*repo.ID, 10)
	f := logrus.Fields{
		"functionName":       "handleRepositoryAddedAction",
		utils.XREQUESTID:     ctx.Value(utils.XREQUESTID),
		"repositoryFullName": repositoryFullName,
	}

	organizationName := strings.Split(repositoryFullName, "/")[0]
	orgModel, err := a.githubOrgRepo.GetGitHubOrganization(ctx, organizationName)
	if err != nil {
		log.Warnf("fetching github org failed : %v", err)
		return nil, err
	}
	orgName := orgModel.OrganizationName

	claGroupID := orgModel.AutoEnabledClaGroupID
	if claGroupID == "" {
		enabled := true
		repos, listErr := a.repositoryService.ListProjectRepositories(context.Background(), orgModel.ProjectSFID, &enabled)
		if listErr != nil {
			log.WithFields(f).Warnf("problem fetching the repositories for orgName : %s for ProjectSFID : %s", orgName, orgModel.ProjectSFID)
			return nil, listErr
		}

		if len(repos.List) == 0 {
			log.WithFields(f).Warnf("no repositories found for orgName : %s, skipping autoEnabled", orgName)
			return nil, ErrCantDetermineAutoEnableClaGroup
		}

		claGroupID, listErr = DetermineClaGroupID(f, orgModel, repos)
		if listErr != nil {
			return nil, listErr
		}
	}
	claGroupModel, err := a.claRepository.GetCLAGroup(ctx, claGroupID)
	if err != nil {
		log.Warnf("fetching the cla group for cla group id : %s failed : %v", claGroupID, err)
		return nil, err
	}

	projectSFID := claGroupModel.ProjectSFID
	if projectSFID == "" {
		projectSFID = orgModel.ProjectSFID
	}

	externalProjectID := claGroupModel.ProjectExternalID
	var repoModel *models.GithubRepository
	existingRepo, err := a.repositoryService.GetRepositoryByExternalID(ctx, repositoryExternalID)
	if err != nil {
		// Expecting Not found - no issue if not found - all other error we throw
		if _, ok := err.(*utils.GitHubRepositoryNotFound); !ok {
			return nil, err
		}
		if !orgModel.AutoEnabled {
			log.Warnf("skipping adding the repository, autoEnabled flag is off")
			return nil, ErrAutoEnabledOff
		}

		repoModel, err = a.gitV1Repository.GitHubAddRepository(ctx, externalProjectID, projectSFID, &models.GithubRepositoryInput{
			RepositoryProjectID:        swag.String(claGroupID),
			RepositoryName:             swag.String(repositoryFullName),
			RepositoryType:             swag.String("github"),
			RepositoryURL:              swag.String("https://github.com/" + repositoryFullName),
			RepositoryOrganizationName: swag.String(organizationName),
			RepositoryExternalID:       swag.String(repositoryExternalID),
		})

		if err != nil {
			return nil, err
		}
	} else {
		// Here repository already exists. We update the same repository with latest document in order to avoid duplicate entries.
		var enabled = false
		if existingRepo.IsRemoteDeleted && existingRepo.WasClaEnforced {
			enabled = true
		} else {
			enabled = existingRepo.Enabled
		}
		repoModel, err = a.gitV1Repository.GitHubUpdateRepository(ctx, existingRepo.RepositoryID, projectSFID, externalProjectID, &models.GithubRepositoryInput{
			RepositoryName:             swag.String(repositoryFullName),
			RepositoryOrganizationName: swag.String(organizationName),
			RepositoryProjectID:        swag.String(claGroupID),
			Enabled:                    &enabled,
			RepositoryType:             swag.String("github"),
			RepositoryURL:              swag.String("https://github.com/" + repositoryFullName),
		})
		if err != nil {
			return nil, err
		}
		if existingRepo.IsRemoteDeleted {
			err = a.gitV1Repository.GitHubSetRemoteDeletedRepository(ctx, existingRepo.RepositoryID, false, false)
			if err != nil {
				return nil, err
			}
		}
	}

	return repoModel, nil
}

func (a *autoEnableServiceProvider) AutoEnabledForGithubOrg(f logrus.Fields, gitHubOrg github_organizations.GithubOrganization, notify bool) error {
	orgName := gitHubOrg.OrganizationName
	log.WithFields(f).Debugf("running AutoEnable for github org : %s", orgName)
	if gitHubOrg.OrganizationInstallationID == 0 {
		log.WithFields(f).Warnf("missing installation id for : %s", orgName)
		return fmt.Errorf("missing installation id")
	}

	enabled := true
	repos, err := a.repositoryService.ListProjectRepositories(context.Background(), gitHubOrg.ProjectSFID, &enabled)
	if err != nil {
		log.WithFields(f).Warnf("problem fetching the repositories for orgName : %s for ProjectSFID : %s", orgName, gitHubOrg.ProjectSFID)
		return err
	}

	if len(repos.List) == 0 {
		log.WithFields(f).Warnf("no repositories found for orgName : %s, skipping autoEnabled", orgName)
		return nil
	}

	claGroupID, err := DetermineClaGroupID(f, github_organizations.ToModel(&gitHubOrg), repos)
	if err != nil {
		return err
	}

	for _, repo := range repos.List {
		if repo.RepositoryClaGroupID == claGroupID {
			continue
		}

		repo.RepositoryClaGroupID = claGroupID
		if err := a.repositoryService.UpdateClaGroupID(context.Background(), repo.RepositoryID, claGroupID); err != nil {
			log.WithFields(f).Warnf("updating claGroupID for repository : %s failed : %v", repo.RepositoryID, err)
			return err
		}
	}

	if notify {
		if err := a.NotifyCLAManagerForRepos(claGroupID, repos.List); err != nil {
			log.Warnf("notifying Cla Managers for Cla Group : %s failed : %v", claGroupID, err)
		}
	}

	return nil
}

func (a *autoEnableServiceProvider) NotifyCLAManagerForRepos(claGroupID string, repos []*models.GithubRepository) error {
	if len(repos) == 0 {
		log.Warnf("NotifyCLAManagerForRepos no repos to notify for, can't continue")
		return nil
	}

	claManagers, err := a.claService.GetCLAManagers(context.Background(), claGroupID)
	if err != nil {
		log.Warnf("NotifyCLAManagerForRepos fetching cla managers failed : %v", err)
		return err
	}

	if len(claManagers) == 0 {
		log.Warnf("no cla managers registered for the claGroup : %s, none to notify", claGroupID)
		return nil
	}

	claGroupModel, err := a.claService.GetCLAGroupByID(context.Background(), claGroupID)
	if err != nil {
		log.Warnf("loading claGroupModel : %s failed : %v", claGroupID, err)
		return err
	}

	// get the emails and send the emails at this stage ...
	subject, body, recipients := autoEnabledRepositoryEmailContent(claGroupModel, repos[0].RepositoryOrganizationName, claManagers, repos)
	if len(recipients) == 0 {
		log.Warnf("no cla manager emails for claGroup : %s registered, can't notify the cla managers ", claGroupModel.ProjectName)
		return nil
	}

	log.Debugf("sending email with subject : %s for claGroup : %s for recipients : %+v", subject, claGroupModel.ProjectName, recipients)
	if err := utils.SendEmail(subject, body, recipients); err != nil {
		log.Warnf("sending email for subject : %s and claGroup : %s failed : %v", subject, claGroupModel.ProjectName, err)
		return err
	}

	return nil
}

// autoEnabledRepositoryEmailContent prepares the email for autoEnabled repositories
func autoEnabledRepositoryEmailContent(claGroupModel *models.ClaGroup, orgName string, managers []*models.ClaManagerUser, repos []*models.GithubRepository) (string, string, []string) {
	claGroupName := claGroupModel.ProjectName
	subject := fmt.Sprintf("EasyCLA: Auto-Enable CombinedRepository for CLA Group: %s", claGroupName)
	repoPronounUpper := "CombinedRepository"
	repoPronoun := "repository"
	pronoun := "this " + repoPronoun
	repoWasHere := repoPronoun + " was"
	if len(repos) > 1 {
		repoPronounUpper = "V3Repositories"
		repoPronoun = "repositories"
		pronoun = "these " + repoPronoun
		repoWasHere = repoPronoun + " were"
	}

	repoContent := "<ul>"
	for _, repo := range repos {
		repoContent += "<li>" + repo.RepositoryName + "</li>"
	}
	repoContent += "</ul>"

	body := `
	<p>Hello Project Manager,</p>
	<p>This is a notification email from EasyCLA regarding the CLA Group %s.</p>
	<p>EasyCLA was notified that the following %s added to the %s GitHub Organization.
	Since auto-enable was configured within EasyCLA for GitHub Organization, the %s will now start enforcing
	CLA checks.</p>
	<p>Please verify the repository settings to ensure EasyCLA is a required check for merging Pull Requests.
See: GitHub CombinedRepository -> Settings -> Branches -> Branch Protection Rules -> Add/Edit the default branch,
	and confirm that 'Require status checks to pass before merging' is enabled and that EasyCLA is a required check.
	Additionally, consider selecting the 'Include administrators' option to enforce all configured restrictions for 
	contributors, maintainers, and administrators.</p>
	<p>For more information on how to setup GitHub required checks, please consult the About required status checks
	<a href="https://docs.github.com/en/github/administering-a-repository/about-required-status-checks"> 
	in the GitHub Online Help Pages</a>.</p>
	<p>%s:</p>
	%s
	%s
	%s
	`

	body = fmt.Sprintf(
		body, claGroupName, repoWasHere,
		orgName, pronoun, repoPronounUpper, repoContent,
		utils.GetEmailHelpContent(claGroupModel.Version == utils.V2), utils.GetEmailSignOffContent())
	var recipients []string
	for _, m := range managers {
		if m.UserEmail == "" {
			continue
		}
		recipients = append(recipients, m.UserEmail)
	}

	return subject, body, recipients
}

// DetermineClaGroupID checks if AutoEnabledClaGroupID is set then returns it (high precedence) otherwise tries to determine
// the autoEnabled claGroupID by guessing from existing repos
func DetermineClaGroupID(f logrus.Fields, gitHubOrg *models.GithubOrganization, repos *models.GithubListRepositories) (string, error) {
	if gitHubOrg.AutoEnabledClaGroupID != "" {
		return gitHubOrg.AutoEnabledClaGroupID, nil
	}

	// fallback to old way of checking the cla group id by guessing it from the existing repos which has cla group id set
	claGroupSet := map[string]bool{}
	sfidSet := map[string]bool{}
	// check if any of the repos is member to more than one cla group, in general shouldn't happen
	var claGroupID string
	for _, repo := range repos.List {
		if repo.RepositoryClaGroupID == "" || repo.RepositoryProjectSfid == "" {
			continue
		}
		claGroupID = repo.RepositoryClaGroupID
		claGroupSet[repo.RepositoryClaGroupID] = true
		sfidSet[repo.RepositoryProjectSfid] = true
	}

	if len(claGroupSet) == 0 && len(sfidSet) == 0 {
		return "", fmt.Errorf("none of the existing repos have the clagroup set, can't determine the cla group, please set the claGroupID on githubOrg : %w", ErrCantDetermineAutoEnableClaGroup)
	}

	if len(claGroupSet) != 1 || len(sfidSet) != 1 {
		log.WithFields(f).Errorf(`Auto Enabled set for Organization %s, '
                                but we found repositories or SFIDs that belong to multiple CLA Groups.
                                Auto Enable only works when all repositories under a given
                                GitHub Organization are associated with a single CLA Group. This
                                organization is associated with %d CLA Groups and %d SFIDs.`, gitHubOrg.OrganizationName, len(claGroupSet), len(sfidSet))
		return "", fmt.Errorf("project and its repos should be part of the same cla group, can't determine main cla group, please set the claGroupID on githubOrg : %w", ErrCantDetermineAutoEnableClaGroup)
	}

	return claGroupID, nil
}
