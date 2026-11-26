// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"errors"

	"github.com/go-openapi/runtime/middleware"
	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations/cla_manager"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	service2 "github.com/linuxfoundation/easycla/cla-backend-go/project/service"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

func companyCLAGroupOrgLens(ctx context.Context, companyService company.IService, projectService service2.Service, lensURL func(ctx context.Context, companySFID, claGroupID string) string, companyID, claGroupID string) (*models.CompanyClaGroupOrgLens, error) {
	companyModel, err := companyService.GetCompany(ctx, companyID)
	if err != nil {
		return nil, err
	}
	if companyModel == nil {
		return nil, &utils.CompanyNotFound{CompanyID: companyID}
	}
	claGroupModel, err := projectService.GetCLAGroupByID(ctx, claGroupID)
	if err != nil {
		return nil, err
	}
	if claGroupModel == nil {
		return nil, &utils.CLAGroupNotFound{CLAGroupID: claGroupID}
	}
	return &models.CompanyClaGroupOrgLens{
		CompanyID:      companyModel.CompanyID,
		CompanySFID:    companyModel.CompanyExternalID,
		ClaGroupID:     claGroupModel.ProjectID,
		FoundationSFID: claGroupModel.FoundationSFID,
		OrgLensURL:     lensURL(ctx, companyModel.CompanyExternalID, claGroupModel.ProjectID),
	}, nil
}

func configureOrgLens(api *operations.ClaAPI, companyService company.IService, projectService service2.Service, lensURL func(ctx context.Context, companySFID, claGroupID string) string) {
	api.ClaManagerGetCompanyCLAGroupOrgLensHandler = cla_manager.GetCompanyCLAGroupOrgLensHandlerFunc(func(params cla_manager.GetCompanyCLAGroupOrgLensParams) middleware.Responder {
		reqID := utils.GetRequestID(params.XREQUESTID)
		ctx := context.WithValue(params.HTTPRequest.Context(), utils.XREQUESTID, reqID) // nolint
		lens, err := companyCLAGroupOrgLens(ctx, companyService, projectService, lensURL, params.CompanyID, params.ClaGroupID)
		if err != nil {
			var companyNotFound *utils.CompanyNotFound
			var claGroupNotFound *utils.CLAGroupNotFound
			if errors.As(err, &companyNotFound) || errors.As(err, &claGroupNotFound) {
				return cla_manager.NewGetCompanyCLAGroupOrgLensNotFound().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{Code: "404", Message: err.Error()})
			}
			log.WithFields(logrus.Fields{
				"functionName":   "cla_manager.handlers.GetCompanyCLAGroupOrgLens",
				utils.XREQUESTID: reqID,
				"companyID":      params.CompanyID,
				"claGroupID":     params.ClaGroupID,
			}).WithError(err).Warn("unable to load the company or the CLA Group")
			return cla_manager.NewGetCompanyCLAGroupOrgLensInternalServerError().WithXRequestID(reqID).WithPayload(&models.ErrorResponse{Code: "500", Message: "unable to load the company or the CLA Group"})
		}
		return cla_manager.NewGetCompanyCLAGroupOrgLensOK().WithXRequestID(reqID).WithPayload(lens)
	})
}
