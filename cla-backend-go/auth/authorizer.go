// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package auth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	swagerrors "github.com/go-openapi/errors"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/communitybridge/easycla/cla-backend-go/user"
)

const (
	projectScope Scope = "project"
	companyScope Scope = "company"
	adminScope   Scope = "admin"
)

// Scope string
type Scope string

// UserPermissioner interface methods
type UserPermissioner interface {
	GetUserAndProfilesByLFID(lfidUsername string) (user.CLAUser, error)
	GetUserProjectIDs(userID string) ([]string, error)
	GetClaManagerCorporateClaIDs(userID string) ([]string, error)
	GetUserCompanyIDs(userID string) ([]string, error)
}

// Authorizer data model
type Authorizer struct {
	authValidator    Validator
	userPermissioner UserPermissioner
}

// NewAuthorizer creates a new authorizer based on the specified parameters
func NewAuthorizer(authValidator Validator, userPermissioner UserPermissioner) Authorizer {
	return Authorizer{
		authValidator:    authValidator,
		userPermissioner: userPermissioner,
	}
}

// SecurityAuth creates a new CLA user based on the token and scopes
func (a Authorizer) SecurityAuth(token string, scopes []string) (*user.CLAUser, error) {
	f := logrus.Fields{
		"functionName": "auth.authorizer.SecurityAuth",
		"scopes":       strings.Join(scopes, ","),
	}
	// This handler is called by the runtime whenever a route needs authentication
	// against the 'OAuthSecurity' scheme.
	// It is passed a token extracted from the Authentication Bearer header, and
	// the list of scopes mentioned by the spec for this route.

	// Verify the token is valid
	// LG:to skip verification
	log.WithFields(f).Debug("verifying token...")
	claims, err := a.authValidator.VerifyToken(token)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("SecurityAuth - verify token error: %+v", err)
		if strings.Contains(strings.ToLower(err.Error()), "expired") {
			return nil, swagerrors.New(401, "%s", err.Error())
		}
		return nil, err
	}
	f["claims"] = fmt.Sprintf("%+v", claims)

	// Get the username from the token claims
	// LG: for V3 endpoints comment this out and set: username, name and email manually for local testing.
	// username, name, email := "user", "Name Surname", "example@gmail.com"
	// username, name, email := "mock-user-go-20250522", "Mock User Go 2025-05-22", "u20250522@mock.user.go.pl"
	usernameClaim, ok := claims[a.authValidator.usernameClaim]
	if !ok {
		log.WithFields(f).Warnf("username not found in claims with key: %s", a.authValidator.usernameClaim)
		return nil, errors.New("username not found")
	}

	username, ok := usernameClaim.(string)
	if !ok {
		log.WithFields(f).Warnf("invalid username: %+v", usernameClaim)
		return nil, errors.New("invalid username")
	}
	f["username"] = username

	nameClaim, ok := claims[a.authValidator.nameClaim]
	if !ok {
		log.WithFields(f).Warnf("name not found: %+v", a.authValidator.nameClaim)
		return nil, errors.New("name not found")
	}
	f["nameClaim"] = nameClaim

	name, ok := nameClaim.(string)
	if !ok {
		log.WithFields(f).Warn("invalid name - not a string")
		return nil, errors.New("invalid name")
	}
	f["name"] = name

	emailClaim, ok := claims[a.authValidator.emailClaim]
	if !ok {
		log.WithFields(f).Warnf("email not found: %+v", a.authValidator.emailClaim)
		return nil, errors.New("email not found")
	}
	email, ok := emailClaim.(string)
	if !ok {
		log.WithFields(f).Warn("SecurityAuth - invalid email - not a string")
		return nil, errors.New("invalid email")
	}
	f["email"] = email
	// LG:end

	// Get User by LFID
	log.WithFields(f).Debugf("loading user and profiles by LFID: %s", username)
	lfuser, err := a.userPermissioner.GetUserAndProfilesByLFID(username)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("GetUserAndProfilesByLFID error fetching username: %s, error: %+v", username, err)
		if err.Error() == "user not found" {
			return &user.CLAUser{
				Name:       name,
				LFEmail:    email,
				LFUsername: username,
			}, nil
		}
		return nil, err
	}
	//log.WithFields(f).Debugf("user loaded : %+v with scopes : %+v", lfuser, scopes)

	for _, scope := range scopes {
		switch Scope(scope) {
		case projectScope:
			log.WithFields(f).Debugf("loading project IDs by username for name: %s, username: %s with email: %s",
				lfuser.Name, lfuser.LFUsername, lfuser.LFEmail)
			projectIDs, err := a.userPermissioner.GetUserProjectIDs(lfuser.LFUsername)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("GetUserProjectIDs error locaing project IDs for user by username: %s, error: %+v", lfuser.LFUsername, err)
				return nil, err
			}

			lfuser.ProjectIDs = projectIDs
		case companyScope:
			//TODO:  Get all companies for this user
			companies, err := a.userPermissioner.GetUserCompanyIDs(lfuser.UserID)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("GetUserCompanyIDs error loading user company IDs for user id: %s, error: %+v", lfuser.UserID, err)
				return nil, err
			}

			lfuser.CompanyIDs = companies
		case adminScope:
		}
	}

	//log.WithFields(f).Debugf("returning user from auth : %+v", lfuser)
	return &lfuser, nil
}
