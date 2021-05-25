// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package projects_cla_groups

import "context"

// ProjectCLAGroupsService interface
type ProjectCLAGroupsService interface {
	GetClaGroupIDForProject(ctx context.Context, projectSFID string) (*ProjectClaGroup, error)
	GetProjectsIdsForClaGroup(ctx context.Context, claGroupID string) ([]*ProjectClaGroup, error)
	GetProjectsIdsForFoundation(ctx context.Context, foundationSFID string) ([]*ProjectClaGroup, error)
	GetProjectsIdsForAllFoundation() ([]*ProjectClaGroup, error)
	AssociateClaGroupWithProject(ctx context.Context, claGroupID string, projectSFID string, foundationSFID string) error
	RemoveProjectAssociatedWithClaGroup(ctx context.Context, claGroupID string, projectSFIDList []string, all bool) error
	GetCLAGroupNameByID(ctx context.Context, claGroupID string) (string, error)
	GetCLAGroup(ctx context.Context, claGroupID string) (*ProjectClaGroup, error)

	IsExistingFoundationLevelCLAGroup(ctx context.Context, foundationSFID string) (bool, error)
	IsAssociated(ctx context.Context, projectSFID string, claGroupID string) (bool, error)
	UpdateRepositoriesCount(ctx context.Context, projectSFID string, diff int64, reset bool) error
	UpdateClaGroupName(ctx context.Context, projectSFID string, claGroupName string) error
}

// Service model
type Service struct {
	repo Repository
}

// NewService creates a new whitelist service
func NewService(repo Repository) Service {
	return Service{
		repo,
	}
}

// GetClaGroupIDForProject service method
func (s Service) GetClaGroupIDForProject(ctx context.Context, projectSFID string) (*ProjectClaGroup, error) {
	return s.repo.GetClaGroupIDForProject(ctx, projectSFID)
}

// GetProjectsIdsForClaGroup service method
func (s Service) GetProjectsIdsForClaGroup(ctx context.Context, claGroupID string) ([]*ProjectClaGroup, error) {
	return s.repo.GetProjectsIdsForClaGroup(ctx, claGroupID)
}

// GetProjectsIdsForFoundation service method
func (s Service) GetProjectsIdsForFoundation(ctx context.Context, foundationSFID string) ([]*ProjectClaGroup, error) {
	return s.repo.GetProjectsIdsForFoundation(ctx, foundationSFID)
}

// GetProjectsIdsForAllFoundation service method
func (s Service) GetProjectsIdsForAllFoundation(ctx context.Context) ([]*ProjectClaGroup, error) {
	return s.repo.GetProjectsIdsForAllFoundation(ctx)
}

// AssociateClaGroupWithProject service method
func (s Service) AssociateClaGroupWithProject(ctx context.Context, claGroupID string, projectSFID string, foundationSFID string) error {
	return s.repo.AssociateClaGroupWithProject(ctx, claGroupID, projectSFID, foundationSFID)
}

// RemoveProjectAssociatedWithClaGroup service method
func (s Service) RemoveProjectAssociatedWithClaGroup(ctx context.Context, claGroupID string, projectSFIDList []string, all bool) error {
	return s.repo.RemoveProjectAssociatedWithClaGroup(ctx, claGroupID, projectSFIDList, all)
}

// GetCLAGroupNameByID service method
func (s Service) GetCLAGroupNameByID(ctx context.Context, claGroupID string) (string, error) {
	return s.repo.GetCLAGroupNameByID(ctx, claGroupID)
}

// GetCLAGroup service method
func (s Service) GetCLAGroup(ctx context.Context, claGroupID string) (*ProjectClaGroup, error) {
	return s.repo.GetCLAGroup(ctx, claGroupID)
}

// IsExistingFoundationLevelCLAGroup service method
func (s Service) IsExistingFoundationLevelCLAGroup(ctx context.Context, foundationSFID string) (bool, error) {
	return s.repo.IsExistingFoundationLevelCLAGroup(ctx, foundationSFID)
}

// IsAssociated service method
func (s Service) IsAssociated(ctx context.Context, projectSFID string, claGroupID string) (bool, error) {
	return s.repo.IsAssociated(ctx, projectSFID, claGroupID)
}

// UpdateRepositoriesCount service method
func (s Service) UpdateRepositoriesCount(ctx context.Context, projectSFID string, diff int64, reset bool) error {
	return s.repo.UpdateRepositoriesCount(ctx, projectSFID, diff, reset)
}

// UpdateClaGroupName service method
func (s Service) UpdateClaGroupName(ctx context.Context, projectSFID string, claGroupName string) error {
	return s.repo.UpdateClaGroupName(ctx, projectSFID, claGroupName)
}
