// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
)

type service struct {
	v1CompanyService     company.IService
	signatureRepo        signatures.SignatureRepository
	projectRepo          ProjectRepo
	userRepo             users.UserRepository
	companyRepo          company.IRepository
	projectClaGroupsRepo projects_cla_groups.Repository
	eventService         events.Service
}

type claGroupModel struct {
	ProjectName  string
	ProjectLogo  string
	ProjectSFID  string
	ProjectType  string
	SubProjects  []string
	ClaGroupName string
	ClaGroupID   string
	IclaEnabled  bool
	CclaEnabled  bool

	// For processing
	FoundationSFID string
	SubProjectIDs  []string
}
