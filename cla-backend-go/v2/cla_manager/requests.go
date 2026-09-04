// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"fmt"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/linuxfoundation/easycla/cla-backend-go/emails"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	sigAPI "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// GetCLAManagerRequests returns the list of CLA manager requests for the given company and CLA group
func (s *service) GetCLAManagerRequests(ctx context.Context, companyModel *v1Models.Company, claGroupID string) (*models.ClaManagerRequestList, error) {
	requestList, err := s.managerService.GetRequests(companyModel.CompanyID, claGroupID)
	if err != nil {
		return nil, err
	}
	result := v2ClaManagerRequestList(requestList)
	// the shared v1 read projection drops company_external_id - enrich from the company model already in hand
	for i := range result.Requests {
		result.Requests[i].CompanyExternalID = companyModel.CompanyExternalID
	}
	return result, nil
}

// GetCLAManagerRequest returns the CLA manager request identified by requestID if it belongs to the given company and CLA group
func (s *service) GetCLAManagerRequest(ctx context.Context, companyModel *v1Models.Company, claGroupID, requestID string) (*models.ClaManagerRequest, error) {
	request, err := s.managerService.GetRequest(requestID)
	if err != nil {
		return nil, err
	}
	if request == nil || request.CompanyID != companyModel.CompanyID || request.ProjectID != claGroupID {
		return nil, errRequestNotFound
	}
	result := v2ClaManagerRequest(request)
	result.CompanyExternalID = companyModel.CompanyExternalID
	return result, nil
}

// ApproveCLAManagerRequest approves the CLA manager request, adds the requester to the CCLA signature ACL and sends the notification emails
func (s *service) ApproveCLAManagerRequest(ctx context.Context, authUser *auth.User, companyModel *v1Models.Company, claGroupID, requestID string) (*models.ClaManagerRequest, error) {
	f := logrus.Fields{
		"functionName":   "v2.cla_manager.service.ApproveCLAManagerRequest",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyModel.CompanyID,
		"claGroupID":     claGroupID,
		"requestID":      requestID,
	}

	existingRequest, err := s.managerService.GetRequest(requestID)
	if err != nil {
		return nil, err
	}
	if existingRequest == nil || existingRequest.CompanyID != companyModel.CompanyID || existingRequest.ProjectID != claGroupID {
		return nil, errRequestNotFound
	}

	claGroupModel, err := s.projectService.GetCLAGroupByID(ctx, claGroupID)
	if err != nil {
		return nil, err
	}
	if claGroupModel == nil {
		return nil, fmt.Errorf("cla group not found for CLA Group ID: %s", claGroupID)
	}

	sigModels, sigErr := s.signatureService.GetProjectCompanySignatures(ctx, sigAPI.GetProjectCompanySignaturesParams{
		HTTPRequest: nil,
		CompanyID:   companyModel.CompanyID,
		ProjectID:   claGroupID,
		NextKey:     nil,
		PageSize:    aws.Int64(5),
	})
	if sigErr != nil || sigModels == nil || len(sigModels.Signatures) == 0 {
		return nil, fmt.Errorf("error reading CCLA Signatures for company ID: %s, CLA Group ID: %s, error: %+v", companyModel.CompanyID, claGroupID, sigErr)
	}
	if len(sigModels.Signatures) > 1 {
		log.WithFields(f).Warnf("returned multiple CCLA signature models for company ID: %s, project ID: %s for request ID: %s",
			companyModel.CompanyID, claGroupID, requestID)
	}

	sigModel := sigModels.Signatures[0]
	claManagers := sigModel.SignatureACL

	request, err := s.managerService.ApproveRequest(companyModel.CompanyID, claGroupID, requestID)
	if err != nil {
		return nil, err
	}

	_, aclErr := s.signatureService.AddCLAManager(ctx, sigModel.SignatureID, request.UserID)
	if aclErr != nil {
		return nil, aclErr
	}

	s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType:  events.ClaManagerAccessRequestApproved,
		ProjectID:  claGroupID,
		CompanyID:  companyModel.CompanyID,
		LfUsername: authUser.UserName,
		UserName:   authUser.UserName,
		EventData: &events.CLAManagerRequestApprovedEventData{
			RequestID:    request.RequestID,
			CompanyName:  companyModel.CompanyName,
			ProjectName:  claGroupModel.ProjectName,
			UserName:     request.UserName,
			UserEmail:    request.UserEmail,
			ManagerName:  authUser.UserName,
			ManagerEmail: authUser.Email,
		},
	})

	for _, manager := range claManagers {
		sendRequestApprovedEmailToCLAManagers(s.emailTemplateService, emails.RequestApprovedToCLAManagersTemplateParams{
			CommonEmailParams: emails.CommonEmailParams{
				RecipientName:    manager.Username,
				RecipientAddress: manager.LfEmail.String(),
				CompanyName:      companyModel.CompanyName,
			},
			RequesterName:  request.UserName,
			RequesterEmail: request.UserEmail,
		}, claGroupModel)
	}

	sendRequestApprovedEmailToRequester(s.emailTemplateService, emails.RequestApprovedToRequesterTemplateParams{
		CommonEmailParams: emails.CommonEmailParams{
			RecipientName:    request.UserName,
			RecipientAddress: request.UserEmail,
			CompanyName:      companyModel.CompanyName,
		},
	}, claGroupModel)

	result := v2ClaManagerRequest(request)
	result.CompanyExternalID = companyModel.CompanyExternalID
	return result, nil
}

// DenyCLAManagerRequest denies the CLA manager request and sends the notification emails
func (s *service) DenyCLAManagerRequest(ctx context.Context, authUser *auth.User, companyModel *v1Models.Company, claGroupID, requestID string) (*models.ClaManagerRequest, error) {
	f := logrus.Fields{
		"functionName":   "v2.cla_manager.service.DenyCLAManagerRequest",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyModel.CompanyID,
		"claGroupID":     claGroupID,
		"requestID":      requestID,
	}

	existingRequest, err := s.managerService.GetRequest(requestID)
	if err != nil {
		return nil, err
	}
	if existingRequest == nil || existingRequest.CompanyID != companyModel.CompanyID || existingRequest.ProjectID != claGroupID {
		return nil, errRequestNotFound
	}

	claGroupModel, err := s.projectService.GetCLAGroupByID(ctx, claGroupID)
	if err != nil {
		return nil, err
	}
	if claGroupModel == nil {
		return nil, fmt.Errorf("cla group not found for CLA Group ID: %s", claGroupID)
	}

	sigModels, sigErr := s.signatureService.GetProjectCompanySignatures(ctx, sigAPI.GetProjectCompanySignaturesParams{
		HTTPRequest: nil,
		CompanyID:   companyModel.CompanyID,
		ProjectID:   claGroupID,
		NextKey:     nil,
		PageSize:    aws.Int64(5),
	})
	if sigErr != nil || sigModels == nil || len(sigModels.Signatures) == 0 {
		return nil, fmt.Errorf("error reading CCLA Signatures for company ID: %s, CLA Group ID: %s, error: %+v", companyModel.CompanyID, claGroupID, sigErr)
	}
	if len(sigModels.Signatures) > 1 {
		log.WithFields(f).Warnf("returned multiple CCLA signature models for company ID: %s, project ID: %s for request ID: %s",
			companyModel.CompanyID, claGroupID, requestID)
	}

	sigModel := sigModels.Signatures[0]
	claManagers := sigModel.SignatureACL

	request, err := s.managerService.DenyRequest(companyModel.CompanyID, claGroupID, requestID)
	if err != nil {
		return nil, err
	}

	s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType:  events.ClaManagerAccessRequestDenied,
		ProjectID:  claGroupID,
		CompanyID:  companyModel.CompanyID,
		LfUsername: authUser.UserName,
		UserName:   authUser.UserName,
		EventData: &events.CLAManagerRequestDeniedEventData{
			RequestID:    request.RequestID,
			CompanyName:  companyModel.CompanyName,
			ProjectName:  claGroupModel.ProjectName,
			UserName:     request.UserName,
			UserEmail:    request.UserEmail,
			ManagerName:  authUser.UserName,
			ManagerEmail: authUser.Email,
		},
	})

	for _, manager := range claManagers {
		sendRequestDeniedEmailToCLAManagers(s.emailTemplateService, emails.RequestDeniedToCLAManagersTemplateParams{
			CommonEmailParams: emails.CommonEmailParams{
				RecipientName:    manager.Username,
				RecipientAddress: manager.LfEmail.String(),
				CompanyName:      companyModel.CompanyName,
			},
			RequesterName:  request.UserName,
			RequesterEmail: request.UserEmail,
		}, claGroupModel)
	}

	sendRequestDeniedEmailToRequester(s.emailTemplateService, emails.CommonEmailParams{
		RecipientName:    request.UserName,
		RecipientAddress: request.UserEmail,
		CompanyName:      companyModel.CompanyName,
	}, claGroupModel)

	result := v2ClaManagerRequest(request)
	result.CompanyExternalID = companyModel.CompanyExternalID
	return result, nil
}

func v2ClaManagerRequest(src *v1Models.ClaManagerRequest) *models.ClaManagerRequest {
	if src == nil {
		return nil
	}
	return &models.ClaManagerRequest{
		RequestID:         src.RequestID,
		CompanyID:         src.CompanyID,
		CompanyExternalID: src.CompanyExternalID,
		CompanyName:       src.CompanyName,
		Created:           src.Created,
		ProjectExternalID: src.ProjectExternalID,
		ProjectID:         src.ProjectID,
		ProjectName:       src.ProjectName,
		Status:            src.Status,
		Updated:           src.Updated,
		UserEmail:         src.UserEmail,
		UserExternalID:    src.UserExternalID,
		UserID:            src.UserID,
		UserName:          src.UserName,
	}
}

func v2ClaManagerRequestList(src *v1Models.ClaManagerRequestList) *models.ClaManagerRequestList {
	result := &models.ClaManagerRequestList{
		Requests: make([]models.ClaManagerRequest, 0),
	}
	if src == nil {
		return result
	}
	for i := range src.Requests {
		result.Requests = append(result.Requests, *v2ClaManagerRequest(&src.Requests[i]))
	}
	return result
}

func sendRequestApprovedEmailToCLAManagers(emailSvc emails.EmailTemplateService, emailParams emails.RequestApprovedToCLAManagersTemplateParams, claGroupModel *v1Models.ClaGroup) {
	projectName := claGroupModel.ProjectName

	subject := fmt.Sprintf("EasyCLA: CLA Manager Access Approval Notice for %s", projectName)
	recipients := []string{emailParams.RecipientAddress}
	body, err := emails.RenderRequestApprovedToCLAManagersTemplate(emailSvc, claGroupModel.Version, claGroupModel.ProjectExternalID, emailParams)
	if err != nil {
		log.Warnf("rendering email template : %s failed : %v", emails.RequestApprovedToCLAManagersTemplateName, err)
		return
	}
	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

func sendRequestApprovedEmailToRequester(emailSvc emails.EmailTemplateService, emailParams emails.RequestApprovedToRequesterTemplateParams, claGroupModel *v1Models.ClaGroup) {
	projectName := claGroupModel.ProjectName

	subject := fmt.Sprintf("EasyCLA: New CLA Manager Access Approved for %s", projectName)
	recipients := []string{emailParams.RecipientAddress}
	body, err := emails.RenderRequestApprovedToRequesterTemplate(emailSvc, claGroupModel.Version, claGroupModel.ProjectExternalID, emailParams)
	if err != nil {
		log.Warnf("email template : %s failed rendering : %s", emails.RequestApprovedToRequesterTemplateName, err)
		return
	}
	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

func sendRequestDeniedEmailToCLAManagers(emailSvc emails.EmailTemplateService, emailParams emails.RequestDeniedToCLAManagersTemplateParams, claGroupModel *v1Models.ClaGroup) {
	projectName := claGroupModel.ProjectName

	subject := fmt.Sprintf("EasyCLA: CLA Manager Access Denied Notice for %s", projectName)
	recipients := []string{emailParams.RecipientAddress}
	body, err := emails.RenderRequestDeniedToCLAManagersTemplate(emailSvc, claGroupModel.Version, claGroupModel.ProjectExternalID, emailParams)
	if err != nil {
		log.Warnf("email template render : %s failed : %v", emails.RequestDeniedToCLAManagersTemplateName, err)
		return
	}
	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}

func sendRequestDeniedEmailToRequester(emailSvc emails.EmailTemplateService, emailParams emails.CommonEmailParams, claGroupModel *v1Models.ClaGroup) {
	projectName := claGroupModel.ProjectName

	subject := fmt.Sprintf("EasyCLA: New CLA Manager Access Denied for %s", projectName)
	recipients := []string{emailParams.RecipientAddress}
	body, err := emails.RenderRequestDeniedToRequesterTemplate(emailSvc, claGroupModel.Version, claGroupModel.ProjectExternalID, emails.RequestDeniedToRequesterTemplateParams{
		CommonEmailParams: emailParams,
	})
	if err != nil {
		log.Warnf("email template rendering %s failed : %v", emails.RequestDeniedToRequesterTemplateName, err)
		return
	}
	err = utils.SendEmail(subject, body, recipients)
	if err != nil {
		log.Warnf("problem sending email with subject: %s to recipients: %+v, error: %+v", subject, recipients, err)
	} else {
		log.Debugf("sent email with subject: %s to recipients: %+v", subject, recipients)
	}
}
