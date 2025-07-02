// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"

	"github.com/LF-Engineering/lfx-kit/auth"

	"github.com/sirupsen/logrus"

	"github.com/gofrs/uuid"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

// NewContext returns a new context object with a new request ID
func NewContext() context.Context {
	f := logrus.Fields{
		"functionName": "utils.NewContext",
	}
	requestID, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to generate a UUID for x-request-id")
		return context.Background()
	}

	return context.WithValue(context.Background(), XREQUESTID, requestID.String()) // nolint
}

// NewContextWithUser returns a new context with a newly generated request ID and the specified user
func NewContextWithUser(authUser *auth.User) context.Context {
	f := logrus.Fields{
		"functionName": "utils.NewContextWithUser",
	}
	requestID, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to generate a UUID for x-request-id")
		return context.Background()
	}

	return context.WithValue(context.WithValue(context.Background(), XREQUESTID, requestID), CtxAuthUser, authUser) // nolint
}

// NewContextFromParent returns a new context object with a new request ID based on the parent
func NewContextFromParent(ctx context.Context) context.Context {
	f := logrus.Fields{
		"functionName": "utils.NewContext",
	}
	requestID, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to generate a UUID for x-request-id")
		return context.Background()
	}

	return context.WithValue(ctx, XREQUESTID, requestID.String()) // nolint
}

// ContextWithRequestAndUser returns a new context with the specified request ID and user
func ContextWithRequestAndUser(ctx context.Context, reqID string, authUser *auth.User) context.Context {
	return context.WithValue(context.WithValue(ctx, XREQUESTID, reqID), CtxAuthUser, authUser) // nolint
}

// ContextWithUser returns a new context with the specified user
func ContextWithUser(ctx context.Context, authUser *auth.User) context.Context {
	return context.WithValue(ctx, "authUser", authUser) // nolint
}

// GetUserNameFromContext returns the user's name from the context
func GetUserNameFromContext(ctx context.Context) string {
	val := ctx.Value(CtxAuthUser)
	if val != nil {
		authUser := val.(*auth.User) // nolint
		if authUser != nil {
			return authUser.UserName
		}
	}

	return ""
}

// GetUserEmailFromContext returns the user's email from the context
func GetUserEmailFromContext(ctx context.Context) string {
	val := ctx.Value(CtxAuthUser)
	if val != nil {
		authUser := val.(*auth.User) // nolint
		if authUser != nil {
			return authUser.Email
		}
	}

	return ""
}
