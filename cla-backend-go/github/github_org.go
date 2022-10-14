// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"errors"
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/google/go-github/v37/github"
)

// errors
var (
	ErrGithubOrganizationNotFound = errors.New("github organization name not found")
)

// GetOrganization gets github organization
func GetMembership(ctx context.Context, user, organizationName string) (*github.Membership, error) {
	f := logrus.Fields{
		"functionName":     "GetOrganization",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"organizationName": organizationName,
	}

	client := NewGithubOauthClient()
	membership, resp, err := client.Organizations.GetOrgMembership(ctx, user, organizationName)

	if err != nil {
		log.WithFields(f).Warnf("GetOrgOrganization %s failed. error = %s", organizationName, err.Error())
		if resp != nil && resp.StatusCode == 404 {
			return nil, ErrGithubOrganizationNotFound
		}
		return nil, err
	}
	return membership, nil
}

// GetOrganization gets github organization
func GetOrganization(ctx context.Context, organizationName string) (*github.Organization, error) {
	f := logrus.Fields{
		"functionName":     "GetOrganization",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"organizationName": organizationName,
	}

	client := NewGithubOauthClient()
	org, resp, err := client.Organizations.Get(ctx, organizationName)
	if err != nil {
		log.WithFields(f).Warnf("GetOrganization %s failed. error = %s", organizationName, err.Error())
		if resp != nil && resp.StatusCode == 404 {
			return nil, ErrGithubOrganizationNotFound
		}
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := fmt.Sprintf("GetOrganization %s failed with no success response code %d. error = %s", organizationName, resp.StatusCode, err.Error())
		log.WithFields(f).Warnf(msg)
		return nil, errors.New(msg)
	}
	return org, nil
}

// GetOrganizationMembers gets members in organization
func GetOrganizationMembers(ctx context.Context, orgName string, installationID int64) ([]string, error) {
	f := logrus.Fields{
		"functionName":   "GetOrganizationMembers",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	client, err := NewGithubAppClient(installationID)
	if err != nil {
		msg := fmt.Sprintf("unable to create a github client, error: %+v", err)
		log.WithFields(f).WithError(err).Warn(msg)
		return nil, errors.New(msg)
	}

	users, resp, err := client.Organizations.ListMembers(ctx, orgName, nil)

	if resp.StatusCode < 200 || resp.StatusCode > 299 || err != nil {
		msg := fmt.Sprintf("List Org Members failed for Organization: %s with no success response code %d. error = %s", orgName, resp.StatusCode, err.Error())
		log.WithFields(f).Warnf(msg)
		return nil, errors.New(msg)
	}

	var ghUsernames []string
	for _, user := range users {
		log.WithFields(f).Debugf("user :%s found for organization: %s", *user.Login, orgName)
		ghUsernames = append(ghUsernames, *user.Login)
	}
	return ghUsernames, nil
}
