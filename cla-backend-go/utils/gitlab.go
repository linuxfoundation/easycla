// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"errors"
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/config"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

type GatedGitlabUser struct {
	*gitlab.User
	Err error
}

var missingCompanyAffiliation = errors.New("must confirm affiliation with their company")

func PrepareMrCommentContent(missingUsers []*GatedGitlabUser, signedUsers []*gitlab.User, signURL string) string {
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

	const startUl = "<ul>"
	const endUl = "<ul>"
	result := ""
	failed := ":x:"
	success := ":white_check_mark:"

	if len(signedUsers) > 0 {
		result += startUl
		for _, signed := range signedUsers {
			authorInfo := GetAuthorInfo(signed)
			result += fmt.Sprintf("<li>%s %s</li>", success, authorInfo)
		}
		result += endUl
		body = coveredBadge
	}

	// gitlabSupportURL := "https://about.gitlab.com/support"
	easyCLASupportURL := "https://jira.linuxfoundation.org/servicedesk/customer/portal/4"
	// faq := "https://docs.linuxfoundation.org/lfx/easycla/v2-current/getting-started/easycla-troubleshooting#github-unable-to-contribute-to-easycla-enforced-repositories"

	if len(missingUsers) > 0 {
		result += startUl
		for _, missingUser := range missingUsers {
			authorInfo := GetAuthorInfo(missingUser.User)
			if errors.Is(missingUser.Err, missingCompanyAffiliation) {
				msg := fmt.Sprintf(`<li> %s %s. This user is authorized, but they must confirm their affiliation with their company. 
								  Start the authorization process <a href='%s'> by clicking here</a>, click "Corporate", 
								  select the appropriate company from the list, then confirm your affiliation on the page that appears.
								  For further assistance with EasyCLA,
								  <a href='%s' target='_blank'>please submit a support request ticket</a>. </li>`, failed, authorInfo, signURL, easyCLASupportURL)
				result += msg
				body = confirmationNeededBadge
			} else {
				msg := fmt.Sprintf(`<li><a href='%s' target='_blank'>%s</a> - %s. The commit is not authorized under a signed CLA.
									<a href='%s' target='_blank'>Please click here to be authorized</a>.
									For further assistance with EasyCLA,
									<a href='%s' target='_blank'>please submit a support request ticket</a>.
									</li>`, signURL, failed, authorInfo, signURL, easyCLASupportURL)
				result += msg
				body = failedBadge
			}
		}
		result += endUl
	}

	if result != "" {
		body += "<br/><br/>" + result
	}

	return body
}

func GetAuthorInfo(gitlabUser *gitlab.User) string {
	f := logrus.Fields{
		"functionName":   "GetAuthorInfo",
		"gitlabUsername": gitlabUser.Username,
		"gitlabName":     gitlabUser.Name,
		"gitlabEmail":    gitlabUser.Email,
	}
	log.WithFields(f).Debug("getting author info")
	if gitlabUser.Username != "" {
		return fmt.Sprintf("login:@%s/name:%s", gitlabUser.Username, gitlabUser.Name)
	} else if gitlabUser.Email != "" {
		return fmt.Sprintf("email:%s/name:%s", gitlabUser.Email, gitlabUser.Name)
	}
	return fmt.Sprintf("name:%s", gitlabUser.Name)
}

func GetFullSignURL(gitlabOrganizationID string, gitlabRepositoryID string, mrID string) string {
	return fmt.Sprintf("%s/v4/repository-provider/%s/sign/%s/%s/%s/#/",
		config.GetConfig().ClaAPIV4Base,
		GitLabLower,
		gitlabOrganizationID,
		gitlabRepositoryID,
		mrID,
	)
}
