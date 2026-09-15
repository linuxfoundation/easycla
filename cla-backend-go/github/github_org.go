// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"

	"github.com/google/go-github/v37/github"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
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
		log.WithFields(f).Warnf("%s", msg)
		return nil, errors.New(msg)
	}
	return org, nil
}

// ListUserPublicOrgs returns the GitHub organization logins that <user> is a
// publicly visible member of. It calls GET /users/<user>/orgs, which is the
// same endpoint the pre-cutover Python helper cla.utils.lookup_github_organizations
// used. Membership in private orgs is invisible to this endpoint unless the
// user has set their membership to public on github.com.
//
// Returns an empty slice (with a nil error) when the user has no visible org
// memberships. The github-org approval-list check must be done against this
// list (case-insensitive) rather than against /orgs/<org>/memberships/<user>,
// because the EasyCLA OAuth bot is not itself a member of customer orgs and
// gets a 403 from the latter endpoint.
//
// An empty user is rejected with an error: go-github routes an empty user
// to GET /user/orgs (the authenticated bot's own orgs), which would silently
// approve unrelated callers if it ever leaked through.
func ListUserPublicOrgs(ctx context.Context, user string) ([]string, error) {
	f := logrus.Fields{
		"functionName":   "github.ListUserPublicOrgs",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"user":           user,
	}

	if strings.TrimSpace(user) == "" {
		return nil, errors.New("ListUserPublicOrgs: user is empty")
	}

	client := NewGithubOauthClient()
	logins := make([]string, 0)
	opt := &github.ListOptions{PerPage: 100}
	for {
		orgs, resp, err := client.Organizations.List(ctx, user, opt)
		if err != nil {
			log.WithFields(f).Warnf("ListUserPublicOrgs %s failed. error = %s", user, err.Error())
			return nil, err
		}
		for _, org := range orgs {
			if org == nil || org.Login == nil {
				continue
			}
			logins = append(logins, *org.Login)
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return logins, nil
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

	return listOrganizationMembers(ctx, client, orgName)
}

// listOrganizationMembers walks every page of GET /orgs/<org>/members (GitHub pages the
// endpoint, 30 members by default). A failure on any page is returned as a failure: a partial
// member set would silently exempt the members past it from the approval list re-check.
func listOrganizationMembers(ctx context.Context, client *github.Client, orgName string) ([]string, error) {
	f := logrus.Fields{
		"functionName":   "github.listOrganizationMembers",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"orgName":        orgName,
	}

	var ghUsernames []string
	opt := &github.ListMembersOptions{ListOptions: github.ListOptions{PerPage: 100, Page: 1}}
	for {
		users, resp, err := client.Organizations.ListMembers(ctx, orgName, opt)
		if err != nil {
			msg := fmt.Sprintf("List Org Members failed for Organization: %s on page %d. error = %s", orgName, opt.Page, err.Error())
			log.WithFields(f).Warnf("%s", msg)
			return nil, errors.New(msg)
		}
		if resp == nil || resp.StatusCode < 200 || resp.StatusCode > 299 {
			statusCode := 0
			if resp != nil {
				statusCode = resp.StatusCode
			}
			msg := fmt.Sprintf("List Org Members failed for Organization: %s on page %d with no success response code %d", orgName, opt.Page, statusCode)
			log.WithFields(f).Warnf("%s", msg)
			return nil, errors.New(msg)
		}

		for _, user := range users {
			if user == nil || user.Login == nil {
				continue
			}
			log.WithFields(f).Debugf("user :%s found for organization: %s", *user.Login, orgName)
			ghUsernames = append(ghUsernames, *user.Login)
		}
		if resp.NextPage == 0 {
			return ghUsernames, nil
		}
		opt.Page = resp.NextPage
	}
}
