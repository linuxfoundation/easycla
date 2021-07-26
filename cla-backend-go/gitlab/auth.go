// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package gitlab

import (
	"github.com/communitybridge/easycla/cla-backend-go/config"
	"github.com/go-resty/resty/v2"
)

// FetchOauthCredentials is responsible for fetching the credentials from gitlab for alredy started Oauth process (access_token, refresh_token)
func FetchOauthCredentials(code string) (*OauthSuccessResponse, error) {
	client := resty.New()
	params := map[string]string{
		"client_id":     config.GetConfig().Gitlab.AppID,
		"client_secret": config.GetConfig().Gitlab.ClientSecret,
		"code":          code,
		"grant_type":    "authorization_code",
		"redirect_uri":  config.GetConfig().Gitlab.RedirectURI,
		//"redirect_uri": "http://localhost:8080/v4/gitlab/oauth/callback",
	}

	resp, err := client.R().
		SetQueryParams(params).
		SetResult(&OauthSuccessResponse{}).
		Post("https://gitlab.com/oauth/token")

	if err != nil {
		return nil, err
	}

	return resp.Result().(*OauthSuccessResponse), nil
}
