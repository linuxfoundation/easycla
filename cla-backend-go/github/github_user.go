// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/logging"

	"github.com/google/go-github/v37/github"
)

// GetUserDetails return github users details
func GetUserDetails(user string) (*github.User, error) {
	client := NewGithubOauthClient()
	userResp, _, err := client.Users.Get(context.TODO(), user)
	if err != nil {
		logging.Warnf("GetUserDetails failed for user : %s, error = %s\n", user, err.Error())
		err = fmt.Errorf("unable to get github info of %s", user)
		return nil, err
	}
	return userResp, nil
}
