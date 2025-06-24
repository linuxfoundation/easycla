// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"errors"
	"fmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
)

const oauthURL = "https://gitlab.com/oauth/token"

// RefreshOauthToken common routine to refresh the GitLab token
func RefreshOauthToken(refreshToken string) (*OauthSuccessResponse, error) {
	gitLabConfig := config.GetConfig().Gitlab
	f := logrus.Fields{
		"functionName": "gitlab.auth.RefreshOauthToken",
		"refreshToken": refreshToken,
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
		"refresh_token": refreshToken,
		"grant_type":    "refresh_token",
		"redirect_uri":  gitLabConfig.RedirectURI,
		//"redirect_uri": "http://localhost:8080/v4/gitlab/oauth/callback",
	}
	resp, err := client.R().
		SetQueryParams(params).
		SetResult(&OauthSuccessResponse{}).
		Post(oauthURL)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error fetching oauth credentials from gitlab")
		return nil, err
	}

	if resp.StatusCode() != 200 {
		log.WithFields(f).Warnf("error refreshing oauth credentials from gitlab - status code: %d", resp.StatusCode())
		return nil, errors.New("error refreshing oauth credentials from gitlab")
	}

	result, ok := resp.Result().(*OauthSuccessResponse)
	if !ok {
		log.WithFields(f).Warnf("error refreshing oauth credentials from gitlab - non success response: %+v", resp)
		return nil, errors.New("error refreshing oauth credentials from gitlab")
	}

	return result, nil
}

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

	resp, err := client.R().
		SetQueryParams(params).
		SetResult(&OauthSuccessResponse{}).
		Post(oauthURL)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem invoking GitLab auth token exchange to: %s", oauthURL)
		return nil, err
	}

	if resp.StatusCode() < 200 || resp.StatusCode() > 299 {
		msg := fmt.Sprintf("problem invoking GitLab auth token exchange to: %s with status code: %d, response: %s", oauthURL, resp.StatusCode(), string(resp.Body()))
		log.WithFields(f).Warn(msg)
		return nil, errors.New(msg)
	}

	result, ok := resp.Result().(*OauthSuccessResponse)
	if !ok {
		log.WithFields(f).Warnf("error fetching oauth credentials from gitlab - non success response: %+v", resp)
		return nil, errors.New("error fetching oauth credentials from gitlab")
	}

	return result, nil
}
