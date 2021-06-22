// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package user

// Service interface defining the public functions
type Service interface { // nolint
	GetUserAndProfilesByLFID(lfidUsername string) (CLAUser, error)
	GetUserProjectIDs(userID string) ([]string, error)
	GetClaManagerCorporateClaIDs(userID string) ([]string, error)
}

type service struct {
	repo RepositoryInterface
}

// NewService creates a new user service
func NewService(repo RepositoryInterface) Service {
	return service{
		repo: repo,
	}
}

// GetUserAndProfilesByLFID returns the user profile when provided the LFID
func (s service) GetUserAndProfilesByLFID(lfidUsername string) (CLAUser, error) {
	user, err := s.repo.GetUserAndProfilesByLFID(lfidUsername)
	if err != nil {
		return CLAUser{}, err
	}

	return user, nil
}

// GetUserProjectIDs returns a list of project IDs associated with the user
func (s service) GetUserProjectIDs(userID string) ([]string, error) {
	projectIDs, err := s.repo.GetUserProjectIDs(userID)
	if err != nil {
		return nil, err
	}

	return projectIDs, nil
}

// GetClaManagerCorporateClaIDs returns a list of corporate CLAs associated with the user
func (s service) GetClaManagerCorporateClaIDs(userID string) ([]string, error) {
	return nil, nil
}
