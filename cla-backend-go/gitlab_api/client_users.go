// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"context"
	"errors"
	"fmt"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
	goGitLab "github.com/xanzy/go-gitlab"
)

// GetUserByName gets a gitlab user object by the given name
func GetUserByName(ctx context.Context, client *goGitLab.Client, name string) (*goGitLab.User, error) {
	f := logrus.Fields{
		"functionName":   "gitlab_api.client_users.GetUserByName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"name":           name,
	}

	users, resp, err := client.Users.ListUsers(&goGitLab.ListUsersOptions{
		ListOptions: goGitLab.ListOptions{
			Page:    0,
			PerPage: 10,
		},
		Username: utils.StringRef(name),
	})

	if err != nil {
		msg := fmt.Sprintf("problem fetching users, error: %+v", err)
		log.WithFields(f).WithError(err).Warn(msg)
		return nil, errors.New(msg)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := fmt.Sprintf("unable to get user using query: %s, status code: %d", name, resp.StatusCode)
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	if len(users) == 0 {
		return nil, nil
	}

	return users[0], nil
}
