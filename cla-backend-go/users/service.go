// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package users

import (
	"errors"
	"strings"

	"github.com/go-openapi/strfmt"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/sirupsen/logrus"
)

// Service interface for users
type Service interface {
	CreateUser(user *models.User, claUser *user.CLAUser) (*models.User, error)
	Save(user *models.UserUpdate, claUser *user.CLAUser) (*models.User, error)
	UpdateUser(userID string, updates map[string]interface{}) (*models.User, error)
	Delete(userID string, claUser *user.CLAUser) error
	GetUser(userID string) (*models.User, error)
	GetUserByLFUserName(lfUserName string) (*models.User, error)
	GetUserByUserName(userName string, fullMatch bool) (*models.User, error)
	GetUserByEmail(userEmail string) (*models.User, error)
	GetUsersByEmail(userEmail string) ([]*models.User, error)
	GetUsersByLFEmail(userEmail string) ([]*models.User, error)
	GetUserByGitHubID(gitHubID string) (*models.User, error)
	GetUserByGitHubUsername(gitlabUsername string) (*models.User, error)
	GetUsersByIdentity(lfUsername string, emails []string, githubIDs []string) ([]*models.User, error)
	GetUserByGitlabID(gitHubID int) (*models.User, error)
	GetUserByGitLabUsername(gitlabUsername string) (*models.User, error)
	SearchUsers(field string, searchTerm string, fullMatch bool) (*models.Users, error)
	UpdateUserCompanyID(userID, companyID, note string) error
	ConvertUserModelToUserCompatModel(*models.User) (*models.UserCompat, error)
}

type service struct {
	repo   UserRepository
	events events.Service
}

// NewService creates a new service
func NewService(repo UserRepository, events events.Service) Service {
	return service{
		repo,
		events,
	}
}

// CreateUser attempts to create a new user based on the specified model
func (s service) CreateUser(user *models.User, claUser *user.CLAUser) (*models.User, error) {
	userModel, err := s.repo.CreateUser(user)
	if err != nil {
		return nil, err
	}

	// System may need to create user accounts
	var lfUser = "easycla_system_user"
	if claUser != nil && claUser.LFUsername != "" {
		lfUser = claUser.LFUsername
	}

	// Create an event - run as a go-routine
	s.events.LogEvent(&events.LogEventArgs{
		EventType:  events.UserCreated,
		UserModel:  userModel,
		LfUsername: lfUser,
		EventData:  &events.UserCreatedEventData{},
	})

	return userModel, nil
}

func (s service) UpdateUser(userID string, updates map[string]interface{}) (*models.User, error) {
	userModel, err := s.repo.UpdateUser(userID, updates)
	if err != nil {
		return nil, err
	}

	// Log the event
	s.events.LogEvent(&events.LogEventArgs{
		EventType: events.UserUpdated,
		UserID:    userID,
		EventData: &events.UserUpdatedEventData{},
	})

	return userModel, nil
}

// Save saves/updates the user record
func (s service) Save(user *models.UserUpdate, claUser *user.CLAUser) (*models.User, error) {
	userModel, err := s.repo.Save(user)
	if err != nil {
		return nil, err
	}

	// Log the event
	s.events.LogEvent(&events.LogEventArgs{
		EventType:  events.UserUpdated,
		UserModel:  userModel,
		LfUsername: claUser.LFUsername,
		EventData:  &events.UserUpdatedEventData{},
	})

	return userModel, nil
}

// Delete deletes the user record
func (s service) Delete(userID string, claUser *user.CLAUser) error {
	if userID == "" {
		return errors.New("userID is empty")
	}
	err := s.repo.Delete(userID)
	if err != nil {
		return err
	}

	// Log the event
	s.events.LogEvent(&events.LogEventArgs{
		EventType: events.UserDeleted,
		UserID:    claUser.UserID,
		EventData: &events.UserDeletedEventData{
			DeletedUserID: userID,
		},
	})

	return nil
}

// GetUser attempts to locate the user by the user id field
func (s service) GetUser(userID string) (*models.User, error) {
	if userID == "" {
		return nil, errors.New("userID is empty")
	}
	return s.repo.GetUser(userID)
}

// GetUserByLFUserName returns the user record associated with the LF Username value
func (s service) GetUserByLFUserName(lfUserName string) (*models.User, error) {
	if lfUserName == "" {
		return nil, errors.New("username is empty")
	}
	return s.repo.GetUserByLFUserName(lfUserName)
}

// GetUserByUserName attempts to locate the user by the user name field
func (s service) GetUserByUserName(userName string, fullMatch bool) (*models.User, error) {
	if userName == "" {
		return nil, errors.New("username is empty")
	}
	return s.repo.GetUserByUserName(userName, fullMatch)
}

// GetUserByEmail fetches the user by email
func (s service) GetUserByEmail(userEmail string) (*models.User, error) {
	if userEmail == "" {
		return nil, errors.New("userEmail is empty")
	}
	return s.repo.GetUserByEmail(userEmail)
}

// GetUsersByEmail fetches the users by email
func (s service) GetUsersByEmail(userEmail string) ([]*models.User, error) {
	if userEmail == "" {
		return nil, errors.New("userEmail is empty")
	}
	return s.repo.GetUsersByEmail(userEmail)
}

// GetUsersByLFEmail fetches the users by email
func (s service) GetUsersByLFEmail(userEmail string) ([]*models.User, error) {
	if userEmail == "" {
		return nil, errors.New("userEmail is empty")
	}
	return s.repo.GetUsersByLFEmail(userEmail)
}

// GetUserByGitHubID fetches the user by GitHub ID
func (s service) GetUserByGitHubID(gitHubID string) (*models.User, error) {
	if gitHubID == "" {
		return nil, errors.New("gitHubID is empty")
	}
	return s.repo.GetUserByGitHubID(gitHubID)
}

// GetUserByGitHubUsername fetches the user by GitHub username
func (s service) GetUserByGitHubUsername(gitHubUsername string) (*models.User, error) {
	if gitHubUsername == "" {
		return nil, errors.New("gitHubUsername is empty")
	}
	return s.repo.GetUserByGitHubUsername(gitHubUsername)
}

// GetUsersByIdentity resolves the union of EasyCLA user records matching ANY of the
// supplied identity keys — LF username, verified email(s), or linked GitHub numeric ID(s) —
// deduplicated by user ID. It only ever uses GSI-backed lookups (lf-username-index,
// lf-email-index, github-id-index); it never table-scans, so email matching is against the
// primary lf_email only — a match that exists solely in a user's secondary user_emails list
// is intentionally not resolved here (see the by-identity endpoint contract).
//
// A lookup that fails or finds nothing for one key is logged and skipped, not fatal: this is a
// "match any" resolver, so one missing key must not fail the others. Returns an empty (non-nil)
// slice when nothing matches.
func (s service) GetUsersByIdentity(lfUsername string, emails []string, githubIDs []string) ([]*models.User, error) {
	f := logrus.Fields{
		"functionName": "users.service.GetUsersByIdentity",
		"lfUsername":   lfUsername,
		"emailCount":   len(emails),
		"githubIDs":    githubIDs,
	}

	byUserID := make(map[string]*models.User)
	add := func(u *models.User) {
		if u != nil && u.UserID != "" {
			if _, seen := byUserID[u.UserID]; !seen {
				byUserID[u.UserID] = u
			}
		}
	}

	if lfUsername != "" {
		if u, err := s.repo.GetUserByLFUserName(lfUsername); err != nil {
			log.WithFields(f).WithError(err).Debugf("no user match for lfUsername: %s", lfUsername)
		} else {
			add(u)
		}
	}

	for _, email := range emails {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			continue
		}
		// lf-email-index is keyed on lf_email; both helpers query that GSI (no scan).
		if u, err := s.repo.GetUserByEmail(email); err != nil {
			log.WithFields(f).WithError(err).Debugf("no user match for email: %s", email)
		} else {
			add(u)
		}
		if us, err := s.repo.GetUsersByLFEmail(email); err != nil {
			log.WithFields(f).WithError(err).Debugf("no lf-email match for email: %s", email)
		} else {
			for _, u := range us {
				add(u)
			}
		}
	}

	for _, githubID := range githubIDs {
		githubID = strings.TrimSpace(githubID)
		if githubID == "" {
			continue
		}
		if u, err := s.repo.GetUserByGitHubID(githubID); err != nil {
			log.WithFields(f).WithError(err).Debugf("no user match for githubID: %s", githubID)
		} else {
			add(u)
		}
	}

	result := make([]*models.User, 0, len(byUserID))
	for _, u := range byUserID {
		result = append(result, u)
	}
	log.WithFields(f).Debugf("resolved %d unique user record(s)", len(result))
	return result, nil
}

// GetUserByGitlabID fetches the user by Gitlab ID
func (s service) GetUserByGitlabID(gitlabID int) (*models.User, error) {
	return s.repo.GetUserByGitlabID(gitlabID)
}

// GetUserByGitLabUsername fetches the user by GitLab username
func (s service) GetUserByGitLabUsername(gitLabUsername string) (*models.User, error) {
	if gitLabUsername == "" {
		return nil, errors.New("gitLabUsername is empty")
	}
	return s.repo.GetUserByGitLabUsername(gitLabUsername)
}

// SearchUsers attempts to locate the user by the searchField and searchTerm fields
func (s service) SearchUsers(searchField string, searchTerm string, fullMatch bool) (*models.Users, error) {
	return s.repo.SearchUsers(searchField, searchTerm, fullMatch)
}

// UpdateUserCompanyID updates the user's company ID
func (s service) UpdateUserCompanyID(userID, companyID, note string) error {
	return s.repo.UpdateUserCompanyID(userID, companyID, note)
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

// ConvertUserModelToUserCompatModel converts User to UserCompat
func (s service) ConvertUserModelToUserCompatModel(user *models.User) (*models.UserCompat, error) {
	userEmails := make([]strfmt.Email, len(user.Emails))
	for i, e := range user.Emails {
		userEmails[i] = strfmt.Email(e)
	}

	var lfEmail *strfmt.Email
	if user.LfEmail != "" {
		lfEmail = &user.LfEmail
	}

	return &models.UserCompat{
		IsSanctioned:       boolPtr(user.IsSanctioned),
		LfEmail:            lfEmail,
		LfSub:              stringPtr(user.LfSub),
		LfUsername:         stringPtr(user.LfUsername),
		Note:               stringPtr(user.Note),
		UserCompanyID:      stringPtr(user.CompanyID),
		UserEmails:         userEmails,
		UserExternalID:     stringPtr(user.UserExternalID),
		UserGithubID:       stringPtr(user.GithubID),
		UserGithubUsername: stringPtr(user.GithubUsername),
		UserGitlabID:       stringPtr(user.GitlabID),
		UserGitlabUsername: stringPtr(user.GitlabUsername),
		UserID:             user.UserID,
		UserLdapID:         nil,
		UserName:           stringPtr(user.Username),
		Version:            "v1",
	}, nil
}
