// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package user

import "context"

type contextKey string

// verifiedCallerKey carries the caller as their verified token stated them, from the
// authentication middleware to the handlers. It is request-scoped and never persisted
// or cached.
const verifiedCallerKey contextKey = "verified-cla-user"

// ContextWithVerifiedCaller returns a context carrying the caller parsed from their
// verified token.
func ContextWithVerifiedCaller(ctx context.Context, claUser *CLAUser) context.Context {
	return context.WithValue(ctx, verifiedCallerKey, claUser)
}

// VerifiedCallerFromContext returns the caller placed on the context by the
// authentication middleware, or nil when the request did not carry a verifiable token.
func VerifiedCallerFromContext(ctx context.Context) *CLAUser {
	claUser, ok := ctx.Value(verifiedCallerKey).(*CLAUser)
	if !ok {
		return nil
	}
	return claUser
}

// CLAUser data model
type CLAUser struct {
	UserID         string
	Name           string
	Emails         []string
	LFEmail        string
	LFUsername     string
	LfidProvider   Provider
	GithubProvider Provider
	ProjectIDs     []string
	ClaIDs         []string
	CompanyIDs     []string
}

// Provider data model
type Provider struct {
	ProviderUserID string
}

// IsAuthorizedForProject checks if user have access of the project {
func (claUser *CLAUser) IsAuthorizedForProject(projectSFID string) bool {
	for _, v := range claUser.ProjectIDs {
		if v == projectSFID {
			return true
		}
	}
	return false
}
