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

func (s *service) refreshGithubEmails(ctx context.Context, userModel *v1Models.User, allowed *my_clas.Identity) *v1Models.User {
	if s.idpTokens == nil || s.githubVerifiedEmails == nil || allowed.LfUsername == "" || userModel.GithubID == "" || !idBelongs(userModel.GithubID, allowed.GithubIDs) {
		return userModel
	}
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
	emails := uniqueLowerEmails(verified)
	if len(emails) == 0 || sameEmails(emails, userModel.Emails) {
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

func uniqueLowerEmails(emails []string) []string {
	seen := make(map[string]struct{}, len(emails))
	unique := make([]string, 0, len(emails))
	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if _, ok := seen[email]; ok || email == "" {
			continue
		}
		seen[email] = struct{}{}
		unique = append(unique, email)
	}
	return unique
}

func sameEmails(emails, stored []string) bool {
	stored = uniqueLowerEmails(stored)
	if len(emails) != len(stored) {
		return false
	}
	for _, email := range stored {
		if !containsFold(emails, email) {
			return false
		}
	}
	return true
}
