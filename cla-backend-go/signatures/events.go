// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
)

func (s service) createEventLogEntries(ctx context.Context, companyModel *models.Company, claGroupModel *models.ClaGroup, userModel *models.User, approvalList *models.ApprovalList, projectSFID string) {
	for _, value := range approvalList.AddEmailApprovalList {
		// Send an event
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:     events.ClaApprovalListUpdated,
			ProjectID:     claGroupModel.ProjectExternalID,
			ClaGroupModel: claGroupModel,
			CompanyID:     companyModel.CompanyID,
			CompanyModel:  companyModel,
			LfUsername:    userModel.LfUsername,
			UserID:        userModel.UserID,
			UserModel:     userModel,
			ProjectSFID:   projectSFID,
			EventData: &events.CLAApprovalListAddEmailData{
				ApprovalListEmail: value,
			},
		})
	}
	for _, value := range approvalList.RemoveEmailApprovalList {
		// Send an event
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:     events.ClaApprovalListUpdated,
			ProjectID:     claGroupModel.ProjectExternalID,
			ClaGroupModel: claGroupModel,
			CompanyID:     companyModel.CompanyID,
			CompanyModel:  companyModel,
			LfUsername:    userModel.LfUsername,
			UserID:        userModel.UserID,
			UserModel:     userModel,
			ProjectSFID:   projectSFID,
			EventData: &events.CLAApprovalListRemoveEmailData{
				ApprovalListEmail: value,
			},
		})
	}
	for _, value := range approvalList.AddDomainApprovalList {
		// Send an event
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:     events.ClaApprovalListUpdated,
			ProjectID:     claGroupModel.ProjectExternalID,
			ClaGroupModel: claGroupModel,
			CompanyID:     companyModel.CompanyID,
			CompanyModel:  companyModel,
			LfUsername:    userModel.LfUsername,
			UserID:        userModel.UserID,
			UserModel:     userModel,
			ProjectSFID:   projectSFID,
			EventData: &events.CLAApprovalListAddDomainData{
				ApprovalListDomain: value,
			},
		})
	}
	for _, value := range approvalList.RemoveDomainApprovalList {
		// Send an event
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:     events.ClaApprovalListUpdated,
			ProjectID:     claGroupModel.ProjectExternalID,
			ClaGroupModel: claGroupModel,
			CompanyID:     companyModel.CompanyID,
			CompanyModel:  companyModel,
			LfUsername:    userModel.LfUsername,
			UserID:        userModel.UserID,
			UserModel:     userModel,
			ProjectSFID:   projectSFID,
			EventData: &events.CLAApprovalListRemoveDomainData{
				ApprovalListDomain: value,
			},
		})
	}
	for _, value := range approvalList.AddGithubUsernameApprovalList {
		// Send an event
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:     events.ClaApprovalListUpdated,
			ProjectID:     claGroupModel.ProjectExternalID,
			ClaGroupModel: claGroupModel,
			CompanyID:     companyModel.CompanyID,
			CompanyModel:  companyModel,
			LfUsername:    userModel.LfUsername,
			UserID:        userModel.UserID,
			UserModel:     userModel,
			ProjectSFID:   projectSFID,
			EventData: &events.CLAApprovalListAddGitHubUsernameData{
				ApprovalListGitHubUsername: value,
			},
		})
	}
	for _, value := range approvalList.RemoveGithubUsernameApprovalList {
		// Send an event
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:     events.ClaApprovalListUpdated,
			ProjectID:     claGroupModel.ProjectExternalID,
			ClaGroupModel: claGroupModel,
			CompanyID:     companyModel.CompanyID,
			CompanyModel:  companyModel,
			LfUsername:    userModel.LfUsername,
			UserID:        userModel.UserID,
			UserModel:     userModel,
			ProjectSFID:   projectSFID,
			EventData: &events.CLAApprovalListRemoveGitHubUsernameData{
				ApprovalListGitHubUsername: value,
			},
		})
	}
	for _, value := range approvalList.AddGithubOrgApprovalList {
		// Send an event
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:     events.ClaApprovalListUpdated,
			ProjectID:     claGroupModel.ProjectExternalID,
			ClaGroupModel: claGroupModel,
			CompanyID:     companyModel.CompanyID,
			CompanyModel:  companyModel,
			LfUsername:    userModel.LfUsername,
			UserID:        userModel.UserID,
			UserModel:     userModel,
			ProjectSFID:   projectSFID,
			EventData: &events.CLAApprovalListAddGitHubOrgData{
				ApprovalListGitHubOrg: value,
			},
		})
	}
	for _, value := range approvalList.RemoveGithubOrgApprovalList {
		// Send an event
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:     events.ClaApprovalListUpdated,
			CLAGroupID:    claGroupModel.ProjectID,
			ProjectID:     claGroupModel.ProjectExternalID,
			ClaGroupModel: claGroupModel,
			CompanyID:     companyModel.CompanyID,
			CompanyModel:  companyModel,
			LfUsername:    userModel.LfUsername,
			UserID:        userModel.UserID,
			UserModel:     userModel,
			ProjectSFID:   projectSFID,
			EventData: &events.CLAApprovalListRemoveGitHubOrgData{
				ApprovalListGitHubOrg: value,
			},
		})
	}
	for _, value := range approvalList.AddGitlabUsernameApprovalList {
		// Send an event
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:     events.ClaApprovalListUpdated,
			ProjectID:     claGroupModel.ProjectExternalID,
			ClaGroupModel: claGroupModel,
			CompanyID:     companyModel.CompanyID,
			CompanyModel:  companyModel,
			LfUsername:    userModel.LfUsername,
			UserID:        userModel.UserID,
			UserModel:     userModel,
			ProjectSFID:   projectSFID,
			EventData: &events.CLAApprovalListAddGitLabUsernameData{
				ApprovalListGitLabUsername: value,
			},
		})
	}
	for _, value := range approvalList.RemoveGitlabUsernameApprovalList {
		// Send an event
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:     events.ClaApprovalListUpdated,
			ProjectID:     claGroupModel.ProjectExternalID,
			ClaGroupModel: claGroupModel,
			CompanyID:     companyModel.CompanyID,
			CompanyModel:  companyModel,
			LfUsername:    userModel.LfUsername,
			UserID:        userModel.UserID,
			UserModel:     userModel,
			ProjectSFID:   projectSFID,
			EventData: &events.CLAApprovalListRemoveGitLabUsernameData{
				ApprovalListGitLabUsername: value,
			},
		})
	}
	for _, value := range approvalList.AddGitlabOrgApprovalList {
		// Send an event
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:     events.ClaApprovalListUpdated,
			ProjectID:     claGroupModel.ProjectExternalID,
			ClaGroupModel: claGroupModel,
			CompanyID:     companyModel.CompanyID,
			CompanyModel:  companyModel,
			LfUsername:    userModel.LfUsername,
			UserID:        userModel.UserID,
			UserModel:     userModel,
			ProjectSFID:   projectSFID,
			EventData: &events.CLAApprovalListAddGitLabGroupData{
				ApprovalListGitLabGroup: value,
			},
		})
	}
	for _, value := range approvalList.RemoveGitlabOrgApprovalList {
		// Send an event
		s.eventsService.LogEventWithContext(ctx, &events.LogEventArgs{
			EventType:     events.ClaApprovalListUpdated,
			CLAGroupID:    claGroupModel.ProjectID,
			ProjectID:     claGroupModel.ProjectExternalID,
			ClaGroupModel: claGroupModel,
			CompanyID:     companyModel.CompanyID,
			CompanyModel:  companyModel,
			LfUsername:    userModel.LfUsername,
			UserID:        userModel.UserID,
			UserModel:     userModel,
			ProjectSFID:   projectSFID,
			EventData: &events.CLAApprovalListRemoveGitLabGroupData{
				ApprovalListGitLabGroup: value,
			},
		})
	}
}
