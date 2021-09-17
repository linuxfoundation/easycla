// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"context"
	"errors"
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/config"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/gitlab"
)

// FetchOauthCredentials is responsible for fetching the credentials from gitlab for alredy started Oauth process (access_token, refresh_token)
func FetchOauthCredentials(code string) (*OauthSuccessResponse, error) {
	gitLabConfig := config.GetConfig().Gitlab
	f := logrus.Fields{
		"functionName": "gitlab.auth.FetchOauthCredentials",
		"code":         code,
		"redirectURI":  config.GetConfig().Gitlab.RedirectURI,
	}

	if len(gitLabConfig.AppClientID) > 4 {
		f["gitLabClientID"] = fmt.Sprintf("%s...%s", gitLabConfig.AppClientID[0:4], gitLabConfig.AppClientID[len(gitLabConfig.AppClientID)-4:])
	} else {
		return nil, errors.New("gitlab application client ID value is not set - value is empty or malformed")
	}
	if len(gitLabConfig.AppClientSecret) > 4 {
		f["gitLabClientSecret"] = fmt.Sprintf("%s...%s", gitLabConfig.AppClientSecret[0:4], gitLabConfig.AppClientSecret[len(gitLabConfig.AppClientSecret)-4:])
	} else {
		return nil, errors.New("gitlab application client secret value is not set - value is empty or malformed")
	}

	// For info on this authorization flow, see: https://docs.gitlab.com/ee/api/oauth2.html#authorization-code-flow
	client := resty.New()
	params := map[string]string{
		"client_id":     gitLabConfig.AppClientID,
		"client_secret": gitLabConfig.AppClientSecret,
		"code":          code,
		"grant_type":    "authorization_code",
		"redirect_uri":  gitLabConfig.RedirectURI,
		//"redirect_uri": "http://localhost:8080/v4/gitlab/oauth/callback",
	}
	url := "https://gitlab.com/oauth/token"
	resp, err := client.R().
		SetQueryParams(params).
		SetResult(&OauthSuccessResponse{}).
		Post(url)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem invoking GitLab auth token exchange to: %s", url)
		return nil, err
	}

	if resp.StatusCode() < 200 || resp.StatusCode() > 299 {
		msg := fmt.Sprintf("problem invoking GitLab auth token exchange to: %s with status code: %d, response: %s", url, resp.StatusCode(), string(resp.Body()))
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	return resp.Result().(*OauthSuccessResponse), nil
}

// FetchOauthToken is responsible for fetching the user credentials from gitlab for alredy started Oauth process (access_token, refresh_token)
func FetchOauthToken(ctx context.Context, code string) (*oauth2.Token, error) {
	gitLabConfig := config.GetConfig().Gitlab
	f := logrus.Fields{
		"functionName": "gitlab.auth.FetchUserOauthCredentials",
		"code":         code,
		"redirectURI":  "https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/gitlab/user/oauth/callback",
	}

	if len(gitLabConfig.AppClientID) > 4 {
		f["gitLabClientID"] = fmt.Sprintf("%s...%s", gitLabConfig.AppClientID[0:4], gitLabConfig.AppClientID[len(gitLabConfig.AppClientID)-4:])
	} else {
		return nil, errors.New("gitlab application client ID value is not set - value is empty or malformed")
	}
	if len(gitLabConfig.AppClientSecret) > 4 {
		f["gitLabClientSecret"] = fmt.Sprintf("%s...%s", gitLabConfig.AppClientSecret[0:4], gitLabConfig.AppClientSecret[len(gitLabConfig.AppClientSecret)-4:])
	} else {
		return nil, errors.New("gitlab application client secret value is not set - value is empty or malformed")
	}

	// For info on this authorization flow, see: https://docs.gitlab.com/ee/api/oauth2.html#authorization-code-flow
	oauth2Config := oauth2.Config{
		ClientID:     gitLabConfig.AppClientID,
		ClientSecret: gitLabConfig.AppClientSecret,
		Endpoint:     gitlab.Endpoint,
	}

	log.WithFields(f).Debugf("Getting token ...")
	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token object")
		return nil, err
	}

	return token, nil
}
