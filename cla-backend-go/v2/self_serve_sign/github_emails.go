// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package self_serve_sign

import (
	"context"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/my_clas"
)

// IDPTokenSource returns the provider access token held for a linked identity of an LF login
type IDPTokenSource interface {
	IdentityAccessToken(ctx context.Context, lfUsername, provider, userID string) (string, error)
}

// Option configures the Self Serve signing service
type Option func(*service)

// WithGithubEmailRefresh makes PrepareSign store the signer's verified GitHub e-mail addresses, as the pull request flow does
func WithGithubEmailRefresh(tokens IDPTokenSource) Option {
	return func(s *service) {
		s.idpTokens = tokens
	}
}

const githubEmailRefreshTimeout = 5 * time.Second

func (s *service) refreshGithubEmails(ctx context.Context, userModel *v1Models.User, allowed *my_clas.Identity) *v1Models.User {
	if s.idpTokens == nil || s.githubVerifiedEmails == nil || allowed.LfUsername == "" || userModel.GithubID == "" ||
		(len(allowed.GithubIDs) > 0 && !idBelongs(userModel.GithubID, allowed.GithubIDs)) {
		return userModel
	}
	ctx, cancel := context.WithTimeout(ctx, githubEmailRefreshTimeout)
	defer cancel()
	f := logrus.Fields{
		"functionName":   "v2.self_serve_sign.service.refreshGithubEmails",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"userID":         userModel.UserID,
	}
	token, err := s.idpTokens.IdentityAccessToken(ctx, allowed.LfUsername, "github", userModel.GithubID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to read the GitHub token of the linked identity - the stored e-mail addresses stay")
		return userModel
	}
	if token == "" {
		return userModel
	}
	verified, err := s.githubVerifiedEmails(ctx, token)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to list the verified GitHub e-mail addresses - the stored e-mail addresses stay")
		return userModel
	}
	added := newEmails(userModel.Emails, verified)
	if len(added) == 0 {
		return userModel
	}
	updated, err := s.usersService.AddUserEmails(userModel.UserID, added)
	if err != nil || updated == nil {
		log.WithFields(f).WithError(err).Warn("unable to store the verified GitHub e-mail addresses on the EasyCLA user record")
		return userModel
	}
	return updated
}

// newEmails returns the verified addresses that are not stored yet
func newEmails(stored, verified []string) []string {
	added := make([]string, 0, len(verified))
	for _, email := range verified {
		if email = strings.ToLower(strings.TrimSpace(email)); email != "" && !containsFold(stored, email) && !containsFold(added, email) {
			added = append(added, email)
		}
	}
	return added
}
