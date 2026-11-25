// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package self_serve_sign

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
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

// stringSet keeps user_emails a DynamoDB string set through the map based user update
type stringSet []string

func (s stringSet) MarshalDynamoDBAttributeValue(av *dynamodb.AttributeValue) error {
	av.SS = aws.StringSlice(s)
	return nil
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
	emails, added := mergeEmails(userModel.Emails, verified)
	if !added {
		return userModel
	}
	updated, err := s.usersService.UpdateUser(userModel.UserID, map[string]interface{}{
		"user_emails":   stringSet(emails),
		"date_modified": time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil || updated == nil {
		log.WithFields(f).WithError(err).Warn("unable to store the verified GitHub e-mail addresses on the EasyCLA user record")
		return userModel
	}
	return updated
}

// mergeEmails returns the stored addresses plus the verified ones that are not stored yet, and whether any was added
func mergeEmails(stored, verified []string) ([]string, bool) {
	merged := make([]string, 0, len(stored)+len(verified))
	for _, email := range stored {
		if email = strings.TrimSpace(email); email != "" && !containsFold(merged, email) {
			merged = append(merged, email)
		}
	}
	kept := len(merged)
	for _, email := range verified {
		if email = strings.ToLower(strings.TrimSpace(email)); email != "" && !containsFold(merged, email) {
			merged = append(merged, email)
		}
	}
	return merged, len(merged) > kept
}
