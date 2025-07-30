// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"errors"
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// updateChangeRequest is a helper function that updates PR - typically after the docusign is completed
func (s service) updateChangeRequest(ctx context.Context, installationID, repositoryID, pullRequestID int64, projectID string) error {
	f := logrus.Fields{
		"functionName":  "v1.signatures.service.updateChangeRequest",
		"repositoryID":  repositoryID,
		"pullRequestID": pullRequestID,
		"projectID":     projectID,
	}

	githubRepository, ghErr := github.GetGitHubRepository(ctx, installationID, repositoryID)
	if ghErr != nil {
		log.WithFields(f).WithError(ghErr).Warn("unable to get github repository")
		return ghErr
	}
	if githubRepository == nil || githubRepository.Owner == nil {
		msg := "unable to get github repository - repository response is nil or owner is nil"
		log.WithFields(f).Warn(msg)
		return errors.New(msg)
	}
	// log.WithFields(f).Debugf("githubRepository: %+v", githubRepository)
	if githubRepository.Name == nil || githubRepository.Owner.Login == nil {
		msg := fmt.Sprintf("unable to get github repository - missing repository name or owner name for repository ID: %d", repositoryID)
		log.WithFields(f).Warn(msg)
		return errors.New(msg)
	}

	var ghOrg *models.GithubOrganization
	if githubRepository.Owner.Login != nil {
		var ghOrgErr error
		ghOrg, ghOrgErr = s.githubOrgService.GetGitHubOrganizationByName(ctx, *githubRepository.Owner.Login)
		if ghOrgErr != nil {
			log.WithFields(f).WithError(ghOrgErr).Warnf("unable to lookup GitHub organization by name: %s - unable to update GitHub status", *githubRepository.Owner.Login)
		}
	}

	gitHubOrgName := utils.StringValue(githubRepository.Owner.Login)
	gitHubRepoName := utils.StringValue(githubRepository.Name)

	// Fetch committers
	log.WithFields(f).Debugf("fetching commit authors for PR: %d using repository owner: %s, repo: %s", pullRequestID, gitHubOrgName, gitHubRepoName)
	authors, latestSHA, authorsErr := github.GetPullRequestCommitAuthors(ctx, installationID, int(pullRequestID), gitHubOrgName, gitHubRepoName)
	if authorsErr != nil {
		log.WithFields(f).WithError(authorsErr).Warnf("unable to get commit authors for %s/%s for PR: %d", gitHubOrgName, gitHubRepoName, pullRequestID)
		return authorsErr
	}
	log.WithFields(f).Debugf("found %d commit authors for %s/%s for PR: %d", len(authors), gitHubOrgName, gitHubRepoName, pullRequestID)

	signed := make([]*github.UserCommitSummary, 0)
	unsigned := make([]*github.UserCommitSummary, 0)

	// triage signed and unsigned users
	log.WithFields(f).Debugf("triaging %d commit authors for PR: %d using repository %s/%s",
		len(authors), pullRequestID, gitHubOrgName, gitHubRepoName)
	for _, userSummary := range authors {

		if !userSummary.IsValid() {
			log.WithFields(f).Debugf("invalid user summary: %+v", *userSummary)
			unsigned = append(unsigned, userSummary)
			continue
		}

		commitAuthorID := userSummary.GetCommitAuthorID()
		commitAuthorUsername := userSummary.GetCommitAuthorUsername()
		commitAuthorEmail := userSummary.GetCommitAuthorEmail()

		log.WithFields(f).Debugf("checking user - sha: %s, user ID: %s, username: %s, email: %s",
			userSummary.SHA, commitAuthorID, commitAuthorUsername, commitAuthorEmail)

		var user *models.User
		var userErr error

		if commitAuthorID != "" {
			log.WithFields(f).Debugf("looking up user by ID: %s", commitAuthorID)
			user, userErr = s.userService.GetUserByGitHubID(commitAuthorID)
			if userErr != nil {
				log.WithFields(f).WithError(userErr).Warnf("unable to get user by github id: %s", commitAuthorID)
			}
			if user != nil {
				log.WithFields(f).Debugf("found user by ID: %s", commitAuthorID)
			}
		}
		if user == nil && commitAuthorUsername != "" {
			log.WithFields(f).Debugf("looking up user by username: %s", commitAuthorUsername)
			user, userErr = s.userService.GetUserByGitHubUsername(commitAuthorUsername)
			if userErr != nil {
				log.WithFields(f).WithError(userErr).Warnf("unable to get user by github username: %s", commitAuthorUsername)
			}
			if user != nil {
				log.WithFields(f).Debugf("found user by username: %s", commitAuthorUsername)
			}
		}
		if user == nil && commitAuthorEmail != "" {
			log.WithFields(f).Debugf("looking up user by email: %s", commitAuthorEmail)
			user, userErr = s.userService.GetUserByEmail(commitAuthorEmail)
			if userErr != nil {
				log.WithFields(f).WithError(userErr).Warnf("unable to get user by user email: %s", commitAuthorEmail)
			}
			if user != nil {
				log.WithFields(f).Debugf("found user by email: %s", commitAuthorEmail)
			}
		}

		if user == nil {
			log.WithFields(f).Debugf("unable to find user for commit author - sha: %s, user ID: %s, username: %s, email: %s",
				userSummary.SHA, commitAuthorID, commitAuthorUsername, commitAuthorEmail)
			unsigned = append(unsigned, userSummary)
			continue
		}

		log.WithFields(f).Debugf("checking to see if user has signed an ICLA or ECLA for project: %s", projectID)
		userSigned, companyAffiliation, signedErr := s.hasUserSigned(ctx, user, projectID)
		if signedErr != nil {
			log.WithFields(f).WithError(signedErr).Warnf("has user signed error - user: %+v, project: %s", user, projectID)
			unsigned = append(unsigned, userSummary)
			continue
		}

		if companyAffiliation != nil {
			userSummary.Affiliated = *companyAffiliation
		}

		if userSigned != nil {
			userSummary.Authorized = *userSigned
			if userSummary.Authorized {
				signed = append(signed, userSummary)
			} else {
				unsigned = append(unsigned, userSummary)
			}
		}
	}

	log.WithFields(f).Debugf("commit authors status => signed: %+v and missing: %+v", signed, unsigned)
	if ghOrg != nil {
		unsigned, signed = github.SkipWhitelistedBots(s.eventsService, ghOrg, gitHubRepoName, projectID, unsigned)
		log.WithFields(f).Debugf("commit authors status after whitelisting bots => signed: %+v and missing: %+v", signed, unsigned)
	}

	// update pull request
	updateErr := github.UpdatePullRequest(ctx, installationID, int(pullRequestID), gitHubOrgName, gitHubRepoName, githubRepository.ID, *latestSHA, signed, unsigned, s.ClaV1ApiURL, s.claLandingPage, s.claLogoURL)
	if updateErr != nil {
		log.WithFields(f).Debugf("unable to update PR: %d", pullRequestID)
		return updateErr
	}

	return nil
}

// hasUserSigned checks to see if the user has signed an ICLA or ECLA for the project, returns:
// false, false, nil if user is not authorized for ICLA or ECLA
// false, false, some error if user is not authorized for ICLA or ECLA - we has some problem looking up stuff
// true, false, nil if user has an ICLA (authorized, but not company affiliation, no error)
// true, true, nil if user has an ECLA (authorized, with company affiliation, no error)
func (s service) hasUserSigned(ctx context.Context, user *models.User, projectID string) (*bool, *bool, error) {
	f := logrus.Fields{
		"functionName": "v1.signatures.service.hasUserSigned",
		"projectID":    projectID,
		"user":         user,
	}
	var hasSigned bool
	var companyAffiliation bool

	approved := true
	signed := true

	// Check for ICLA
	log.WithFields(f).Debugf("checking to see if user has signed an ICLA")
	signature, sigErr := s.signatureService.GetIndividualSignature(ctx, projectID, user.UserID, &approved, &signed)
	if sigErr != nil {
		log.WithFields(f).WithError(sigErr).Warnf("problem checking for ICLA signature for user: %s", user.UserID)
		return &hasSigned, &companyAffiliation, sigErr
	}
	if signature != nil {
		hasSigned = true
		log.WithFields(f).Debugf("ICLA signature check passed for user: %+v on project : %s", user, projectID)
		return &hasSigned, &companyAffiliation, nil // ICLA passes, no company affiliation
	} else {
		log.WithFields(f).Debugf("ICLA signature check failed for user: %+v on project: %s - ICLA not signed", user, projectID)
	}

	// Check for Employee Acknowledgment ECLA
	companyID := user.CompanyID
	log.WithFields(f).Debugf("checking to see if user has signed a ECLA for company: %s", companyID)

	if companyID != "" {
		companyAffiliation = true

		// Get employee signature
		log.WithFields(f).Debugf("ECLA signature check - user has a company: %s - looking for user's employee acknowledgement...", companyID)

		// Load the company - make sure it is valid
		companyModel, compModelErr := s.companyService.GetCompany(ctx, companyID)
		if compModelErr != nil {
			log.WithFields(f).WithError(compModelErr).Warnf("problem looking up company: %s", companyID)
			return &hasSigned, &companyAffiliation, compModelErr
		}

		// Load the CLA Group - make sure it is valid
		claGroupModel, claGroupModelErr := s.claGroupService.GetCLAGroup(ctx, projectID)
		if claGroupModelErr != nil {
			log.WithFields(f).WithError(claGroupModelErr).Warnf("problem looking up project: %s", projectID)
			return &hasSigned, &companyAffiliation, claGroupModelErr
		}

		employeeSigned, err := s.signatureService.ProcessEmployeeSignature(ctx, companyModel, claGroupModel, user)

		if err != nil {
			log.WithFields(f).WithError(err).Warnf("problem looking up employee signature for company: %s", companyID)
			return &hasSigned, &companyAffiliation, err
		}
		if employeeSigned != nil {
			hasSigned = *employeeSigned
		}

	} else {
		log.WithFields(f).Debugf("ECLA signature check - user does not have a company ID assigned - skipping...")
	}

	return &hasSigned, &companyAffiliation, nil
}
