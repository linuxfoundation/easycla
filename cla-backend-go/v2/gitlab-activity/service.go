// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab_activity

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/v2/common"

	"github.com/communitybridge/easycla/cla-backend-go/config"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	signatures1 "github.com/communitybridge/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	gitlab_api "github.com/communitybridge/easycla/cla-backend-go/gitlab_api"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/repositories"
	"github.com/communitybridge/easycla/cla-backend-go/signatures"
	"github.com/communitybridge/easycla/cla-backend-go/users"
	"github.com/communitybridge/easycla/cla-backend-go/v2/gitlab_organizations"
	gitV2Repositories "github.com/communitybridge/easycla/cla-backend-go/v2/repositories"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

var (
	missingID                 = errors.New("user missing in easyCLA records")
	missingCompanyAffiliation = errors.New("must confirm affiliation with their company")
	missingCompanyApproval    = errors.New("missing in company approval lists")
	secretTokenMismatch       = errors.New("secret token mismatch")
)

// ProcessMergeActivityInput is used to pass the data needed to trigger a gitlab mr check
type ProcessMergeActivityInput struct {
	ProjectName      string
	ProjectPath      string
	ProjectNamespace string
	ProjectID        int
	MergeID          int
	RepositoryPath   string
	LastCommitSha    string
}

type gatedGitlabUser struct {
	*gitlab.User
	err error
}

type Service interface {
	ProcessMergeOpenedActivity(ctx context.Context, secretToken string, mergeEvent *gitlab.MergeEvent) error
	ProcessMergeActivity(ctx context.Context, secretToken string, input *ProcessMergeActivityInput) error
	IsUserApprovedForSignature(ctx context.Context, f logrus.Fields, corporateSignature *models.Signature, user *models.User, gitlabUser *gitlab.User) bool
}

type service struct {
	usersRepository             users.UserRepository
	gitlabRepository            gitlab_organizations.RepositoryInterface
	gitRepository               repositories.RepositoryInterface
	gitV2Repository             gitV2Repositories.RepositoryInterface
	signaturesRepository        signatures.SignatureRepository
	projectsCLAGroupsRepository projects_cla_groups.Repository
	companyRepository           company.IRepository
	signatureRepository         signatures.SignatureRepository
	gitLabApp                   *gitlab_api.App
}

func NewService(gitlabRepository gitlab_organizations.RepositoryInterface, gitRepository repositories.RepositoryInterface, gitV2Repository gitV2Repositories.RepositoryInterface, usersRepository users.UserRepository, signaturesRepository signatures.SignatureRepository, projectsCLAGroupsRepository projects_cla_groups.Repository,
	companyRepository company.IRepository, signatureRepository signatures.SignatureRepository) Service {
	return &service{
		gitlabRepository:            gitlabRepository,
		gitRepository:               gitRepository,
		gitV2Repository:             gitV2Repository,
		usersRepository:             usersRepository,
		signaturesRepository:        signaturesRepository,
		projectsCLAGroupsRepository: projectsCLAGroupsRepository,
		companyRepository:           companyRepository,
		signatureRepository:         signatureRepository,
		gitLabApp:                   gitlab_api.Init(config.GetConfig().Gitlab.AppClientID, config.GetConfig().Gitlab.AppClientSecret, config.GetConfig().Gitlab.AppPrivateKey),
	}
}

func (s service) ProcessMergeOpenedActivity(ctx context.Context, secretToken string, mergeEvent *gitlab.MergeEvent) error {
	projectName := mergeEvent.Project.Name
	projectPath := mergeEvent.Project.PathWithNamespace
	projectNamespace := mergeEvent.Project.Namespace
	projectID := mergeEvent.Project.ID
	mergeID := mergeEvent.ObjectAttributes.IID
	repositoryPath := mergeEvent.Project.PathWithNamespace
	lastCommitSha := mergeEvent.ObjectAttributes.LastCommit.ID

	input := &ProcessMergeActivityInput{
		ProjectName:      projectName,
		ProjectPath:      projectPath,
		ProjectNamespace: projectNamespace,
		ProjectID:        projectID,
		MergeID:          mergeID,
		RepositoryPath:   repositoryPath,
		LastCommitSha:    lastCommitSha,
	}

	return s.ProcessMergeActivity(ctx, secretToken, input)

}

func (s *service) ProcessMergeActivity(ctx context.Context, secretToken string, input *ProcessMergeActivityInput) error {
	projectName := input.ProjectName
	projectPath := input.ProjectPath
	projectNamespace := input.ProjectNamespace
	projectID := input.ProjectID
	mergeID := input.MergeID
	repositoryPath := input.RepositoryPath
	lastCommitSha := input.LastCommitSha

	f := logrus.Fields{
		"functionName":      "ProcessMergeActivity",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"gitlabProjectName": projectName,
		"gitlabProjectID":   projectID,
		"repositoryName":    repositoryPath,
		"mergeID":           mergeID,
	}

	log.WithFields(f).Debugf("looking up for gitlab org in easycla records ...")
	gitlabOrg, err := s.getGitlabOrganizationFromProjectPath(ctx, projectPath, projectNamespace)
	if err != nil {
		return fmt.Errorf("fetching internal gitlab org for following path : %s failed : %v", repositoryPath, err)
	}

	log.WithFields(f).Debugf("checking gitlab org : %s auth state agains the webhook secret token", gitlabOrg.OrganizationName)
	if gitlabOrg.AuthState != secretToken {
		return secretTokenMismatch
	}

	log.WithFields(f).Debugf("internal gitlab org : %s:%s is associated with external path : %s", gitlabOrg.OrganizationID, gitlabOrg.OrganizationName, repositoryPath)

	gitlabClient, err := gitlab_api.NewGitlabOauthClient(gitlabOrg.AuthInfo, s.gitLabApp)
	if err != nil {
		return fmt.Errorf("initializing gitlab client : %v", err)
	}

	_, err = gitlab_api.FetchMrInfo(gitlabClient, projectID, mergeID)
	if err != nil {
		return fmt.Errorf("fetching info for mr : %d and project : %d: %s, failed : %v", mergeID, projectID, projectName, err)
	}

	// try to find the repository via the external id
	gitlabRepo, err := s.getGitlabRepoByName(ctx, repositoryPath)
	if err != nil {
		return fmt.Errorf("finding internal repository for gitlab org name failed : %v", err)
	}

	log.WithFields(f).Debugf("internal gitlab repository found with id : %s", gitlabRepo.RepositoryID)
	participants, err := gitlab_api.FetchMrParticipants(gitlabClient, projectID, mergeID, true)
	if err != nil {
		return fmt.Errorf("fetching mr participants : %v", err)
	}

	if len(participants) == 0 {
		return fmt.Errorf("no participants found in gitlab mr : %d, and gitlab project : %d", mergeID, projectID)
	}

	claGroup, err := s.projectsCLAGroupsRepository.GetClaGroupIDForProject(ctx, gitlabOrg.ProjectSFID)
	if err != nil {
		return fmt.Errorf("fetching claGroup id for gitlabOrg project sfid : %s, failed : %v", gitlabOrg.ProjectSFID, err)
	}
	claGroupID := claGroup.ClaGroupID
	log.WithFields(f).Debugf("gitlabOrg : %s is associated with cla group id : %s", gitlabOrg.OrganizationName, claGroupID)

	log.WithFields(f).Debugf("found following participants for the MR : %d", len(participants))
	missingCLAMsg := "Missing CLA Authorization"
	signedCLAMsg := "EasyCLA check passed. You are authorized to contribute."

	var missingUsers []*gatedGitlabUser
	var signedUsers []*gitlab.User
	for _, gitlabUser := range participants {
		if ok, err := s.hasUserSigned(ctx, claGroupID, gitlabUser); ok {
			log.WithFields(f).Infof("gitlabUser : %d:%s has signed", gitlabUser.ID, gitlabUser.Username)
			signedUsers = append(signedUsers, gitlabUser)
		} else {
			missingUsers = append(missingUsers, &gatedGitlabUser{
				User: gitlabUser,
				err:  err,
			})
			log.WithFields(f).Errorf("gitlabUser : %d:%s hasn't signed, err : %v", gitlabUser.ID, gitlabUser.Username, err)
		}
	}

	signURL := GetFullSignURL(gitlabOrg.OrganizationID, strconv.Itoa(int(gitlabRepo.RepositoryExternalID)), strconv.Itoa(mergeID))
	mrCommentContent := PrepareMrCommentContent(missingUsers, signedUsers, signURL)
	if len(missingUsers) > 0 {
		log.WithFields(f).Errorf("mr faild with following users : %s", mrCommentContent)
		if err := gitlab_api.SetCommitStatus(gitlabClient, projectID, lastCommitSha, gitlab.Failed, missingCLAMsg, signURL); err != nil {
			return fmt.Errorf("setting commit status failed : %v", err)
		}

		if err := gitlab_api.SetMrComment(gitlabClient, projectID, mergeID, mrCommentContent); err != nil {
			return fmt.Errorf("setting comment failed : %v", err)
		}

		return nil
	}
	err = gitlab_api.SetCommitStatus(gitlabClient, projectID, lastCommitSha, gitlab.Success, signedCLAMsg, "")
	if err != nil {
		return fmt.Errorf("setting commit status failed : %v", err)
	}

	if err := gitlab_api.SetMrComment(gitlabClient, projectID, mergeID, mrCommentContent); err != nil {
		return fmt.Errorf("setting comment failed : %v", err)
	}
	return err
}

func PrepareMrCommentContent(missingUsers []*gatedGitlabUser, signedUsers []*gitlab.User, signURL string) string {
	landingPage := config.GetConfig().CLALandingPage
	landingPage += "/#/?version=2"

	var badgeHyperlink string
	if len(missingUsers) > 0 {
		badgeHyperlink = signURL
	} else {
		badgeHyperlink = landingPage
	}

	coveredBadge := fmt.Sprintf(`<a href="%s">
	<img src="https://s3.amazonaws.com/cla-project-logo-dev/cla-signed.svg" alt="CLA Signed" align="left" height="28" width="328" ></a><br/>`, badgeHyperlink)
	failedBadge := fmt.Sprintf(`<a href="%s">
<img src="https://s3.amazonaws.com/cla-project-logo-dev/cla-not-signed.svg" alt="CLA Not Signed" align="left" height="28" width="328" ></a><br/>`, badgeHyperlink)
	// 	missingUserIDBadge := fmt.Sprintf(`<a href="%s">
	// <img src="https://s3.amazonaws.com/cla-project-logo-dev/cla-missing-id.svg" alt="CLA Missing ID" align="left" height="28" width="328" ></a><br/>`, badgeHyperlink)
	confirmationNeededBadge := fmt.Sprintf(`<a href="%s">
<img src="https://s3.amazonaws.com/cla-project-logo-dev/cla-confirmation-needed.svg" alt="CLA Confirmation Needed" align="left" height="28" width="328" ></a><br/>`, badgeHyperlink)

	var body string

	var result string
	failed := ":x:"
	success := ":white_check_mark:"

	if len(signedUsers) > 0 {
		result = "<ul>"
		for _, signed := range signedUsers {
			authorInfo := getAuthorInfo(signed)
			result += fmt.Sprintf("<li>%s %s</li>", success, authorInfo)
		}
		result += "</ul>"
		body = coveredBadge
	}

	// gitlabSupportURL := "https://about.gitlab.com/support"
	easyCLASupportURL := "https://jira.linuxfoundation.org/servicedesk/customer/portal/4"
	if len(missingUsers) > 0 {
		result += "<ul>"
		for _, missingUser := range missingUsers {
			authorInfo := getAuthorInfo(missingUser.User)
			if errors.Is(missingUser.err, missingCompanyAffiliation) {
				msg := fmt.Sprintf(`<li>%s is authorized, but they must confirm their affiliation with their company.
                            Start the authorization process 
                            <a href='%s'> by clicking here</a>, click "Corporate",
                            select the appropriate company from the list, then confirm
                            your affiliation on the page that appears.
                            For further assistance with EasyCLA,
                            <a href='%s' target='_blank'>please submit a support request ticket</a>.
                            </li>`, authorInfo, signURL, easyCLASupportURL)
				result += msg
				body = confirmationNeededBadge
			} else {
				msg := fmt.Sprintf(`<li><a href='%s'>%s</a> - 
							%s's commit is not authorized under a signed CLA. 
                            <a href='%s'>Please click here to be authorized</a>.
                            For further assistance with EasyCLA,
                            <a href='%s' target='_blank'>please submit a support request ticket</a>.
                            </li>`, signURL, failed, authorInfo, signURL, easyCLASupportURL)
				result += msg
				body = failedBadge
			}
		}
		result += "</ul>"
	}

	if result != "" {
		body += "<br/><br/>" + result
	}

	return body
}

func GetFullSignURL(gitlabOrganizationID string, gitlabRepositoryID string, mrID string) string {
	return fmt.Sprintf("%s/v4/repository-provider/%s/sign/%s/%s/%s/#/",
		config.GetConfig().ClaAPIV4Base,
		utils.GitLabLower,
		gitlabOrganizationID,
		gitlabRepositoryID,
		mrID,
	)
}

func getAuthorInfo(gitlabUser *gitlab.User) string {
	return fmt.Sprintf("%d:%s", gitlabUser.ID, gitlabUser.Username)
}

func (s service) getGitlabOrganizationFromProjectPath(ctx context.Context, projectPath, projectNameSpace string) (*common.GitLabOrganization, error) {
	parts := strings.Split(projectPath, "/")
	organizationName := parts[0]

	gitlabOrg, err := s.gitlabRepository.GetGitLabOrganizationByName(ctx, organizationName)
	if err != nil || gitlabOrg == nil {
		// try getting it with project name as well
		gitlabOrg, err = s.gitlabRepository.GetGitLabOrganizationByName(ctx, projectNameSpace)
		if err != nil || gitlabOrg == nil {
			return nil, fmt.Errorf("gitlab org : %s doesn't exist : %v", organizationName, err)
		}
	}

	gitlabOrg, err = s.gitlabRepository.GetGitLabOrganization(ctx, gitlabOrg.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("fetching gitlab org : %s failed : %v", gitlabOrg.OrganizationID, err)
	}

	return gitlabOrg, nil
}

func (s service) getGitlabRepoByName(ctx context.Context, repoNameWithPath string) (*models.GithubRepository, error) {
	gitlabRepo, err := s.gitV2Repository.GitLabGetRepositoryByName(ctx, repoNameWithPath)
	if err != nil || gitlabRepo == nil {
		return nil, fmt.Errorf("unable to locate GitLab repo for repoNameWithPath : %s, failed : %v", repoNameWithPath, err)
	}

	return gitlabRepo.ToGitHubModel(), nil
}

func (s service) hasUserSigned(ctx context.Context, claGroupID string, gitlabUser *gitlab.User) (bool, error) {
	f := logrus.Fields{
		"functionName":    "hasUserSigned",
		utils.XREQUESTID:  ctx.Value(utils.XREQUESTID),
		"gitlabUserID":    gitlabUser.ID,
		"gitlabUserName":  gitlabUser.Username,
		"gitlabUserEmail": gitlabUser.Email,
	}

	userModel, b, err := s.findUserModelForGitlabUser(f, gitlabUser)
	if err != nil {
		return b, err
	}

	if userModel == nil {
		msg := fmt.Sprintf("gitlab user : %d:%s not found in easycla records", gitlabUser.ID, gitlabUser.Username)
		log.WithFields(f).Error(msg)
		return false, missingID
	}
	log.WithFields(f).Debugf("found following easyCLA user for gitlab record, userID: %s, lfusername : %s", userModel.UserID, userModel.LfUsername)

	icla, err := s.signaturesRepository.GetIndividualSignature(ctx, claGroupID, userModel.UserID, aws.Bool(true), aws.Bool(true))
	if err != nil {
		return false, fmt.Errorf("fetching ICLS for gitlab user : %d:%s failed : %v", gitlabUser.ID, gitlabUser.Username, err)
	}

	if icla != nil {
		log.WithFields(f).Infof("user has signed the following signature : %s, passing", icla.SignatureID)
		return true, nil
	}

	if userModel.CompanyID == "" {
		log.WithFields(f).Debugf("user does not have association with any company, can't continue")
		return false, fmt.Errorf("user hasn't signed yet")
	}

	companyID := userModel.CompanyID
	_, err = s.companyRepository.GetCompany(ctx, companyID)
	if err != nil {
		msg := fmt.Sprintf("can't load company record : %s for user : %s association : %v", companyID, userModel.UserID, err)
		log.WithFields(f).Errorf(msg)
		return false, fmt.Errorf(msg)
	}

	corporateSignature, err := s.signatureRepository.GetCorporateSignature(ctx, claGroupID, companyID, aws.Bool(true), aws.Bool(true))
	if err != nil {
		msg := fmt.Sprintf("can't load company signature record : %s for user : %s association : %v", companyID, userModel.UserID, err)
		log.WithFields(f).Errorf(msg)
		return false, fmt.Errorf(msg)
	}

	if corporateSignature == nil {
		msg := fmt.Sprintf("no ccla signature record for company : %s ", companyID)
		log.WithFields(f).Errorf(msg)
		return false, fmt.Errorf(msg)
	}

	log.WithFields(f).Debugf("loaded signature id : %s for claGroupID : %s and companyID : %s", corporateSignature.SignatureID, claGroupID, companyID)

	approvalCriteria := &signatures.ApprovalCriteria{}
	if gitlabUser.Email != "" {
		approvalCriteria.UserEmail = gitlabUser.Email
	} else if gitlabUser.Username != "" {
		approvalCriteria.GitlabUsername = gitlabUser.Username
	} else {
		msg := fmt.Sprintf("gitlabUser model doesn't have enough information to fetch the employee signatures for user : %s", userModel.UserID)
		log.WithFields(f).Errorf(msg)
		return false, fmt.Errorf(msg)
	}
	if !s.IsUserApprovedForSignature(ctx, f, corporateSignature, userModel, gitlabUser) {
		log.WithFields(f).Debugf("user is not approved in signature : %s", corporateSignature.SignatureID)
		return false, missingCompanyApproval
	}

	employeeSignatures, err := s.signaturesRepository.GetProjectCompanyEmployeeSignatures(ctx, signatures1.GetProjectCompanyEmployeeSignaturesParams{
		CompanyID: companyID,
		ProjectID: claGroupID,
	}, approvalCriteria, 100)

	if err != nil {
		msg := fmt.Sprintf("can't load employee signature records : %s for user : %s association : %v", companyID, userModel.UserID, err)
		log.WithFields(f).Errorf(msg)
		return false, fmt.Errorf(msg)
	}

	if len(employeeSignatures.Signatures) == 0 {
		msg := fmt.Sprintf("no employee signature records found for company : %s user : %s association", companyID, userModel.UserID)
		log.WithFields(f).Errorf(msg)
		return false, missingCompanyAffiliation
	}

	log.WithFields(f).Warnf("is in signature approval list : %s and has employee signature", corporateSignature.SignatureID)
	return true, nil
}

func (s service) findUserModelForGitlabUser(f logrus.Fields, gitlabUser *gitlab.User) (*models.User, bool, error) {
	log.WithFields(f).Debugf("Looking up Gitlab user via gitlabID")
	userModel, err := s.usersRepository.GetUserByGitlabID(gitlabUser.ID)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return nil, false, fmt.Errorf("looking up gitlab user via gitlabID : %d failed : %v", gitlabUser.ID, err)
		}
		userModel = nil
	}
	if userModel == nil && gitlabUser.Username != "" {
		log.WithFields(f).Debugf("Looking up Gitlab user via user gitlab username")
		userModel, err = s.usersRepository.GetUserByGitlabUsername(gitlabUser.Username)
		if !strings.Contains(err.Error(), "not found") {
			return nil, false, fmt.Errorf("looking up gitlab user via gitlabUsername : %s failed : %v", gitlabUser.Username, err)
		}
	}

	if userModel == nil && gitlabUser.Email != "" {
		log.WithFields(f).Debugf("Looking up Gitlab user via user email")
		userModel, err = s.usersRepository.GetUserByEmail(gitlabUser.Email)
		if err != nil {
			if !errors.Is(err, &utils.UserNotFound{}) {
				return nil, false, fmt.Errorf("looking up gitlab user via email : %s failed : %v", gitlabUser.Email, err)
			}
		}
	}
	return userModel, false, nil
}

func (s service) IsUserApprovedForSignature(ctx context.Context, f logrus.Fields, corporateSignature *models.Signature, user *models.User, gitlabUser *gitlab.User) bool {
	log.WithFields(f).Debugf("checking if user : %s is approved for corporate signature : %s", user.UserID, corporateSignature.SignatureID)
	userEmails := user.Emails
	if string(user.LfEmail) != "" {
		userEmails = append(userEmails, string(user.LfEmail))
	}

	emailApprovalList := corporateSignature.EmailApprovalList
	domainApprovalList := corporateSignature.DomainApprovalList
	log.WithFields(f).Debugf("checking if user : %s is approved for corporate signature : %s, email approval list : %+v", user.UserID, corporateSignature.SignatureID, emailApprovalList)

	if len(userEmails) > 0 && len(emailApprovalList) > 0 {
		for _, email := range userEmails {
			for _, approvalEmail := range emailApprovalList {
				if email == approvalEmail {
					log.WithFields(f).Debugf("found user email : %s in email approval list ", email)
					return true
				}
			}
		}
	} else {
		log.WithFields(f).Warnf("no match for user in signature email approval list")
	}

	if len(domainApprovalList) > 0 && len(userEmails) > 0 {
		log.WithFields(f).Debugf("checking if emails : %+v are approved for corporate signature : %s, domain approval list : %+v", userEmails, corporateSignature.SignatureID, domainApprovalList)
		for _, userEmail := range userEmails {
			for _, domainApprovalPattern := range domainApprovalList {
				if strings.HasPrefix(domainApprovalPattern, "*.") {
					domainApprovalPattern = strings.Replace(domainApprovalPattern, "*.", ".*", 1)
				} else if strings.HasPrefix(domainApprovalPattern, "*") {
					domainApprovalPattern = strings.Replace(domainApprovalPattern, "*", ".*", 1)
				} else if strings.HasPrefix(domainApprovalPattern, ".") {
					domainApprovalPattern = strings.Replace(domainApprovalPattern, ".", ".*", 1)
				}
				regexpApprovalPattern := "^.*@" + domainApprovalPattern + "$"
				if ok, err := regexp.MatchString(regexpApprovalPattern, userEmail); ok && err == nil {
					log.WithFields(f).Debugf("found user email : %s in email approval list : %s", userEmail, domainApprovalPattern)
					return true
				}
			}
		}
	}

	gitlabUserName := gitlabUser.Username
	gitlabUsernameApprovalList := corporateSignature.GitlabUsernameApprovalList
	if gitlabUserName != "" && len(gitlabUsernameApprovalList) > 0 {
		log.WithFields(f).Debugf("checking gitlab username : %s for gitlab approval list : %+v", gitlabUserName, gitlabUsernameApprovalList)
		for _, gitlabApproval := range gitlabUsernameApprovalList {
			if gitlabApproval == gitlabUserName {
				log.WithFields(f).Debugf("found gitlab username : %s in gitlab approval list ", gitlabUserName)
				return true
			}
		}

	} else {
		log.WithFields(f).Warnf("no match found for gitlabUser : %s in gitlab approval list : %+v", gitlabUserName, gitlabUsernameApprovalList)
	}

	gitlabGroupApprovalList := corporateSignature.GitlabOrgApprovalList
	if gitlabUserName != "" && len(gitlabGroupApprovalList) > 0 {
		log.WithFields(f).Debugf("checking gitlab username : %s for gitlab org approval list : %+v ", gitlabUserName, gitlabGroupApprovalList)

		for _, gitlabGroupApproval := range gitlabGroupApprovalList {
			isApproved, err := s.checkGitLabGroupApproval(ctx, gitlabUserName, gitlabGroupApproval)
			if err != nil {
				log.WithFields(f).WithError(err).Warn("unable to get username")
				break
			}
			if isApproved == true {
				log.WithFields(f).Debugf(" found gitlab username : %s in gitlab org approval list : %+v", gitlabUserName, gitlabGroupApprovalList)
				return true
			}
		}
	}

	log.WithFields(f).Errorf("unable to find user in any approval list")
	return false

}

/**
 * Parses url with the given regular expression and returns the
 * group values defined in the expression.
 *
 */
func getParams(regEx, url string) (paramsMap map[string]string) {

	var compRegEx = regexp.MustCompile(regEx)
	match := compRegEx.FindStringSubmatch(url)

	paramsMap = make(map[string]string)
	for i, name := range compRegEx.SubexpNames() {
		if i > 0 && i <= len(match) {
			paramsMap[name] = match[i]
		}
	}
	return paramsMap
}

func (s service) checkGitLabGroupApproval(ctx context.Context, userName, URL string) (bool, error) {
	f := logrus.Fields{
		"functionName": "checkGitLabGroupApproval",
		"userName":     userName,
		"group_url":    URL,
	}

	log.WithFields(f).Debugf("checking approval list gitlab org criteria : %s for user: %s ", URL, userName)
	var searchURL = URL
	params := getParams(`(?P<base>\bhttps://gitlab.com/\b)(?P<group>\bgroups\/\b)?(?P<name>\w+)`, URL)
	if params[`group`] == "" {
		params[`group`] = "groups/"
		updated := fmt.Sprintf("%s%s%s", params[`base`], params[`group`], params[`name`])
		log.WithFields(f).Debugf("updating url : %s to %s for easycla search purporses ", searchURL, updated)
		searchURL = updated
	}
	gitlabOrg, _ := s.gitlabRepository.GetGitLabOrganizationByURL(ctx, searchURL)
	if gitlabOrg != nil {
		gitlabClient, clientErr := gitlab_api.NewGitlabOauthClient(gitlabOrg.AuthInfo, s.gitLabApp)
		if clientErr != nil {
			log.WithFields(f).WithError(clientErr).Warnf("problem getting gitLabClient for org: %s ", gitlabOrg.OrganizationName)
			return false, clientErr
		}
		members, err := gitlab_api.ListGroupMembers(ctx, gitlabClient, int(gitlabOrg.ExternalGroupID))
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem getting gitlab group members")
			return false, err
		}
		for _, member := range members {
			if userName == member.Username {
				log.WithFields(f).Debugf("%s is a member of group: %s ", userName, URL)
				return true, nil
			}
		}
	}

	return false, nil
}
