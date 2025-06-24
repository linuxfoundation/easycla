// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"errors"
	"fmt"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/go-openapi/runtime/middleware"
)

// validateApprovalListInput is a helper function to validate the update approval list input parameters
func validateApprovalListInput(reqID string, params signatures.UpdateApprovalListParams) middleware.Responder {
	if !hasApprovalListUpdates(params) {
		return signatures.NewUpdateApprovalListBadRequest().WithPayload(errorResponse(reqID, errors.New("missing approval list items")))
	}

	msg, valid := entriesAreValid(params)
	if !valid {
		return signatures.NewUpdateApprovalListBadRequest().WithPayload(errorResponse(reqID, errors.New(msg)))
	}
	return nil
}

// hasApprovalListUpdates returns true if we have something to update, otherwise returns false
func hasApprovalListUpdates(params signatures.UpdateApprovalListParams) bool {
	if len(params.Body.AddEmailApprovalList) > 0 || len(params.Body.RemoveEmailApprovalList) > 0 ||
		len(params.Body.AddDomainApprovalList) > 0 || len(params.Body.RemoveDomainApprovalList) > 0 ||
		len(params.Body.AddGithubUsernameApprovalList) > 0 || len(params.Body.RemoveGithubUsernameApprovalList) > 0 ||
		len(params.Body.AddGithubOrgApprovalList) > 0 || len(params.Body.RemoveGithubOrgApprovalList) > 0 ||
		len(params.Body.AddGitlabUsernameApprovalList) > 0 || len(params.Body.RemoveGitlabUsernameApprovalList) > 0 ||
		len(params.Body.AddGitlabOrgApprovalList) > 0 || len(params.Body.RemoveGitlabOrgApprovalList) > 0 {
		return true
	}

	return false
}

// entriesAreValid returns true if the values in the approval list are valid, returns false and a message otherwise
func entriesAreValid(params signatures.UpdateApprovalListParams) (string, bool) {
	var listOfErrors []string
	isValid := true
	// Ensure the email address are valid
	for _, email := range params.Body.AddEmailApprovalList {
		if !utils.ValidEmail(email) {
			isValid = false
			listOfErrors = append(listOfErrors, fmt.Sprintf("invalid add approval list email %s", email))
		}
	}
	for _, email := range params.Body.RemoveEmailApprovalList {
		if !utils.ValidEmail(email) {
			isValid = false
			listOfErrors = append(listOfErrors, fmt.Sprintf("invalid remove approval list email %s", email))
		}
	}

	// Ensure the domains are valid
	for _, domain := range params.Body.AddDomainApprovalList {
		msg, valid := utils.ValidDomain(domain, true)
		if !valid {
			isValid = false
			listOfErrors = append(listOfErrors, fmt.Sprintf("invalid add approval list domain %s - %s", domain, msg))
		}
	}
	for _, domain := range params.Body.RemoveDomainApprovalList {
		msg, valid := utils.ValidDomain(domain, true)
		if !valid {
			isValid = false
			listOfErrors = append(listOfErrors, fmt.Sprintf("invalid remove approval list domain %s - %s", domain, msg))
		}
	}

	// Ensure the GitHub usernames are valid
	for _, githubUsername := range params.Body.AddGithubUsernameApprovalList {
		msg, valid := utils.ValidGitHubUsername(githubUsername)
		if !valid {
			isValid = false
			listOfErrors = append(listOfErrors, fmt.Sprintf("invalid add approval list GitHub Username %s - %s", githubUsername, msg))
		}
	}
	for _, githubUsername := range params.Body.RemoveGithubUsernameApprovalList {
		msg, valid := utils.ValidGitHubUsername(githubUsername)
		if !valid {
			isValid = false
			listOfErrors = append(listOfErrors, fmt.Sprintf("invalid remove approval list GitHub Username %s - %s", githubUsername, msg))
		}
	}

	// Ensure the GitHub Organization values are valid
	for _, githubOrg := range params.Body.AddGithubOrgApprovalList {
		msg, valid := utils.ValidGitHubOrg(githubOrg)
		if !valid {
			isValid = false
			listOfErrors = append(listOfErrors, fmt.Sprintf("invalid add approval list GitHub Org %s - %s", githubOrg, msg))
		}
	}
	for _, githubOrg := range params.Body.RemoveGithubOrgApprovalList {
		msg, valid := utils.ValidGitHubOrg(githubOrg)
		if !valid {
			isValid = false
			listOfErrors = append(listOfErrors, fmt.Sprintf("invalid remove approval list GitHub Org %s - %s", githubOrg, msg))
		}
	}

	// Ensure the Gitlab usernames are valid
	for _, githubUsername := range params.Body.AddGitlabUsernameApprovalList {
		msg, valid := utils.ValidGitlabUsername(githubUsername)
		if !valid {
			isValid = false
			listOfErrors = append(listOfErrors, fmt.Sprintf("invalid add approval list Gitlab Username %s - %s", githubUsername, msg))
		}
	}
	for _, githubUsername := range params.Body.RemoveGitlabUsernameApprovalList {
		msg, valid := utils.ValidGitlabUsername(githubUsername)
		if !valid {
			isValid = false
			listOfErrors = append(listOfErrors, fmt.Sprintf("invalid remove approval list Gitlab Username %s - %s", githubUsername, msg))
		}
	}

	// Ensure the Gitlab Organization values are valid
	for _, githubOrg := range params.Body.AddGitlabOrgApprovalList {
		msg, valid := utils.ValidGitlabOrg(githubOrg)
		if !valid {
			isValid = false
			listOfErrors = append(listOfErrors, fmt.Sprintf("invalid add approval list Gitlab Org %s - %s", githubOrg, msg))
		}
	}
	for _, githubOrg := range params.Body.RemoveGitlabOrgApprovalList {
		msg, valid := utils.ValidGitlabOrg(githubOrg)
		if !valid {
			isValid = false
			listOfErrors = append(listOfErrors, fmt.Sprintf("invalid remove approval list Gitlab Org %s - %s", githubOrg, msg))
		}
	}

	return strings.Join(listOfErrors, ", "), isValid
}
