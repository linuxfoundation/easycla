// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/v2/cla_groups"

	"github.com/sirupsen/logrus"

	acsService "github.com/communitybridge/easycla/cla-backend-go/v2/acs-service"
	"github.com/communitybridge/easycla/cla-backend-go/v2/organization-service/client/organizations"

	organizationService "github.com/communitybridge/easycla/cla-backend-go/v2/organization-service"

	projectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"
	userService "github.com/communitybridge/easycla/cla-backend-go/v2/user-service"

	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
)

// constants
const (
	DontLoadRepoDetails = false
)

// errors
var (
	ErrCCLANotEnabled        = errors.New("corporate license agreement is not enabled with this project")
	ErrTemplateNotConfigured = errors.New("cla template not configured for this project")
	ErrNotInOrg              error
)

// ProjectRepo contains project repo methods
type ProjectRepo interface {
	GetCLAGroupByID(ctx context.Context, claGroupID string, loadRepoDetails bool) (*v1Models.ClaGroup, error)
}

// Service interface defines the sign service methods
type Service interface {
	RequestCorporateSignature(ctx context.Context, lfUsername string, authorizationHeader string, input *models.CorporateSignatureInput) (*models.CorporateSignatureOutput, error)
	RequestIndividualSignature(ctx context.Context, input *models.IndividualSignatureInput) (*models.IndividualSignatureOutput, error)
}

// service
type service struct {
	ClaV1ApiURL          string
	companyRepo          company.IRepository
	projectRepo          ProjectRepo
	projectClaGroupsRepo projects_cla_groups.Repository
	companyService       company.IService
	claGroupService      cla_groups.Service
}

// NewService returns an instance of v2 project service
func NewService(apiURL string, compRepo company.IRepository, projectRepo ProjectRepo, pcgRepo projects_cla_groups.Repository, compService company.IService, claGroupService cla_groups.Service) Service {
	return &service{
		ClaV1ApiURL:          apiURL,
		companyRepo:          compRepo,
		projectRepo:          projectRepo,
		projectClaGroupsRepo: pcgRepo,
		companyService:       compService,
		claGroupService:      claGroupService,
	}
}

type requestCorporateSignatureInput struct {
	ProjectID         string `json:"project_id,omitempty"`
	CompanyID         string `json:"company_id,omitempty"`
	SendAsEmail       bool   `json:"send_as_email,omitempty"`
	SigningEntityName string `json:"signing_entity_name,omitempty"`
	AuthorityName     string `json:"authority_name,omitempty"`
	AuthorityEmail    string `json:"authority_email,omitempty"`
	ReturnURL         string `json:"return_url,omitempty"`
}

type requestCorporateSignatureOutput struct {
	ProjectID   string `json:"project_id"`
	CompanyID   string `json:"company_id"`
	SignatureID string `json:"signature_id"`
	SignURL     string `json:"sign_url"`
}

func (in *requestCorporateSignatureOutput) toModel() *models.CorporateSignatureOutput {
	return &models.CorporateSignatureOutput{
		SignURL:     in.SignURL,
		SignatureID: in.SignatureID,
	}
}

func validateCorporateSignatureInput(input *models.CorporateSignatureInput) error {
	if input.SendAsEmail {
		log.Debugf("input.AuthorityName validation %s", input.AuthorityName)
		if strings.TrimSpace(input.AuthorityName) == "" {
			log.Warn("error in input.AuthorityName ")
			return errors.New("require authority_name")
		}
		if input.AuthorityEmail == "" {
			return errors.New("require authority_email")
		}
	} else {
		if input.ReturnURL.String() == "" {
			return errors.New("require return_url")
		}
	}
	if input.ProjectSfid == nil || *input.ProjectSfid == "" {
		return errors.New("require project_sfid")
	}
	if input.CompanySfid == nil || *input.CompanySfid == "" {
		return errors.New("require company_sfid")
	}
	return nil
}

func (s *service) RequestCorporateSignature(ctx context.Context, lfUsername string, authorizationHeader string, input *models.CorporateSignatureInput) (*models.CorporateSignatureOutput, error) { // nolint
	f := logrus.Fields{
		"functionName":      "sign.RequestCorporateSignature",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"lfUsername":        lfUsername,
		"projectSFID":       input.ProjectSfid,
		"companySFID":       input.CompanySfid,
		"signingEntityName": input.SigningEntityName,
		"authorityName":     input.AuthorityName,
		"authorityEmail":    input.AuthorityEmail.String(),
		"sendAsEmail":       input.SendAsEmail,
		"returnURL":         input.ReturnURL,
	}
	usc := userService.GetClient()

	log.WithFields(f).Debug("validating input parameters...")
	err := validateCorporateSignatureInput(input)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to validat corporate signature input")
		return nil, err
	}

	var comp *v1Models.Company
	// Backwards compatible - if the signing entity name is not set, then we fall back to using the CompanySFID lookup
	// which will return the company record where the company name == signing entity name
	if input.SigningEntityName == "" {
		comp, err = s.companyRepo.GetCompanyByExternalID(ctx, utils.StringValue(input.CompanySfid))
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to fetch company records by signing entity name value")
			return nil, err
		}
	} else {
		// Big change here - since we can have multiple EasyCLA Company records with the same external SFID, we now
		// switch over to query by the signing entity name.
		comp, err = s.companyRepo.GetCompanyBySigningEntityName(ctx, input.SigningEntityName)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to fetch company records by signing entity name value")
			return nil, err
		}
	}

	psc := projectService.GetClient()
	log.WithFields(f).Debug("looking up project by SFID...")
	project, err := psc.GetProject(utils.StringValue(input.ProjectSfid))
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch project SFID")
		return nil, err
	}

	var claGroupID string
	if !utils.IsProjectHaveParent(project) || utils.IsProjectHasRootParent(project) || utils.GetProjectParentSFID(project) == "" {
		// this is root project
		cgmlist, perr := s.projectClaGroupsRepo.GetProjectsIdsForFoundation(ctx, utils.StringValue(input.ProjectSfid))
		if perr != nil {
			log.WithFields(f).WithError(err).Warn("unable to lookup other projects associated with this project SFID")
			return nil, perr
		}
		if len(cgmlist) == 0 {
			// no cla group is link with root_project
			return nil, projects_cla_groups.ErrProjectNotAssociatedWithClaGroup
		}
		claGroups := utils.NewStringSet()
		for _, cg := range cgmlist {
			claGroup, claGroupErr := s.claGroupService.GetCLAGroup(ctx, cg.ClaGroupID)
			if err != nil {
				log.WithFields(f).WithError(claGroupErr).Warn("unable to lookup cla group")
				return nil, err
			}

			// ensure that cla group for project is a foundation level cla group
			if claGroup != nil && cg.ProjectSFID == utils.StringValue(input.ProjectSfid) {
				claGroups.Add(cg.ClaGroupID)
			}
		}

		if claGroups.Length() > 1 {
			// multiple cla group are linked with root_project
			// so we can not determine which cla-group to use
			return nil, errors.New("invalid project_sfid. multiple cla-groups are associated with this project_sfid")
		}
		claGroupID = (claGroups.List())[0]

	} else {
		cgm, perr := s.projectClaGroupsRepo.GetClaGroupIDForProject(ctx, utils.StringValue(input.ProjectSfid))
		if perr != nil {
			log.WithFields(f).WithError(err).Warn("unable to lookup CLA Group ID for this project SFID")
			return nil, perr
		}
		claGroupID = cgm.ClaGroupID
	}

	f["claGroupID"] = claGroupID
	log.WithFields(f).Debug("loading CLA Group by ID...")
	proj, err := s.projectRepo.GetCLAGroupByID(ctx, claGroupID, DontLoadRepoDetails)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to lookup CLA Group by CLA Group ID")
		return nil, err
	}
	if !proj.ProjectCCLAEnabled {
		log.WithFields(f).Warn("unable to request corporate signature - CCLA is not enabled for this CLA Group")
		return nil, ErrCCLANotEnabled
	}
	if len(proj.ProjectCorporateDocuments) == 0 {
		log.WithFields(f).Warn("unable to request corporate signature - missing corporate documents in the CLA Group configuration")
		return nil, ErrTemplateNotConfigured
	}

	// Email flow
	if input.SendAsEmail {
		log.WithFields(f).Debugf("Sending request as an email to: %s...", input.AuthorityEmail.String())
		// this would be used only in case of cla-signatory
		err = prepareUserForSigning(ctx, input.AuthorityEmail.String(), utils.StringValue(input.CompanySfid), utils.StringValue(input.ProjectSfid), input.SigningEntityName)
		if err != nil {
			// Ignore conflict - role has already been assigned
			if _, ok := err.(*organizations.CreateOrgUsrRoleScopesConflict); !ok {
				return nil, err
			}
		}
	} else {
		// Direct to DocuSign flow...
		var currentUserEmail string

		log.WithFields(f).Debugf("Loading user by username: %s...", lfUsername)
		userModel, userErr := usc.GetUserByUsername(lfUsername)
		if userErr != nil {
			return nil, userErr
		}

		if userModel != nil {
			for _, email := range userModel.Emails {
				if email != nil && *email.IsPrimary {
					currentUserEmail = *email.EmailAddress
				}
			}
		}

		err = prepareUserForSigning(ctx, currentUserEmail, utils.StringValue(input.CompanySfid), utils.StringValue(input.ProjectSfid), input.SigningEntityName)
		if err != nil {
			// Ignore conflict - role has already been assigned
			if _, ok := err.(*organizations.CreateOrgUsrRoleScopesConflict); !ok {
				return nil, err
			}
		}
	}

	log.WithFields(f).Debug("Forwarding request to v1 API for requestCorporateSignature...")
	out, err := requestCorporateSignature(authorizationHeader, s.ClaV1ApiURL, &requestCorporateSignatureInput{
		ProjectID:         proj.ProjectID,
		CompanyID:         comp.CompanyID,
		SigningEntityName: input.SigningEntityName,
		SendAsEmail:       input.SendAsEmail,
		AuthorityName:     input.AuthorityName,
		AuthorityEmail:    input.AuthorityEmail.String(),
		ReturnURL:         input.ReturnURL.String(),
	})
	if err != nil {
		if input.AuthorityEmail.String() != "" {
			// remove role
			removeErr := removeSignatoryRole(ctx, input.AuthorityEmail.String(), utils.StringValue(input.CompanySfid), utils.StringValue(input.ProjectSfid))
			if removeErr != nil {
				log.WithFields(f).WithError(removeErr).Warnf("failed to remove signatory role. companySFID :%s, email :%s error: %+v", *input.CompanySfid, input.AuthorityEmail.String(), removeErr)
			}
		}
		return nil, err
	}

	// Update the company ACL
	log.WithFields(f).Debugf("Adding user with LFID: %s to company access list...", lfUsername)
	companyACLError := s.companyService.AddUserToCompanyAccessList(ctx, comp.CompanyID, lfUsername)
	if companyACLError != nil {
		log.WithFields(f).WithError(companyACLError).Warnf("Unable to add user with LFID: %s to company ACL, companyID: %s", lfUsername, *input.CompanySfid)
	}

	return out.toModel(), nil
}

func (s *service) RequestIndividualSignature(ctx context.Context, input *models.IndividualSignatureInput) (*models.IndividualSignatureOutput, error) {
	f := logrus.Fields{
		"functionName":   "sign.RequestIndividualSignature",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectID":      input.ProjectID,
		"returnURL":      input.ReturnURL,
		"returnURLType":  input.ReturnURLType,
		"userID":         input.UserID,
	}

	log.WithFields(f).Debug("Get Access Token for DocuSign")
	accessToken, err := s.getAccessToken(ctx)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to get access token")
		return nil, err
	}
	log.WithFields(f).Debugf("access token: %s", accessToken)
	return nil, nil
}

func requestCorporateSignature(authToken string, apiURL string, input *requestCorporateSignatureInput) (*requestCorporateSignatureOutput, error) {
	f := logrus.Fields{
		"functionName":      "requestCorporateSignature",
		"apiURL":            apiURL,
		"CompanyID":         input.CompanyID,
		"ProjectID":         input.ProjectID,
		"SigningEntityName": input.SigningEntityName,
		"AuthorityName":     input.AuthorityName,
		"AuthorityEmail":    input.AuthorityEmail,
		"ReturnURL":         input.ReturnURL,
		"SendAsEmail":       input.SendAsEmail,
	}
	log.WithFields(f).Debug("Processing request...")
	requestBody, err := json.Marshal(input)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem marshalling input request - error: %+v", err)
		return nil, err
	}

	client := http.Client{}
	log.WithFields(f).Debugf("requesting corporate signatures: %#v", string(requestBody))
	req, err := http.NewRequest("POST", apiURL+"/v1/request-corporate-signature", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authToken)
	resp, err := client.Do(req)
	if err != nil {
		log.WithFields(f).Warnf("client request error: %+v", err)
		return nil, err
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			log.WithFields(f).Warnf("error closing response body: %+v", closeErr)
		}
	}()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithFields(f).Warnf("error reading response body: %+v", err)
		return nil, err
	}
	log.WithFields(f).Debugf("corporate signature response: %#v\n", string(responseBody))
	log.WithFields(f).Debugf("corporate signature response headers :%#v\n", resp.Header)

	if strings.Contains(string(responseBody), "Company has already signed CCLA with this project") {
		log.WithFields(f).Warnf("response contains error: %+v", responseBody)
		return nil, errors.New("company has already signed CCLA with this project")
	} else if strings.Contains(string(responseBody), "Contract Group does not support CCLAs.") {
		log.WithFields(f).Warnf("response contains error: %+v", responseBody)
		return nil, errors.New("contract Group does not support CCLAs")
	} else if strings.Contains(string(responseBody), "user_error': 'user does not exist") {
		log.WithFields(f).Warnf("response contains error: %+v", responseBody)
		return nil, errors.New("user_error': 'user does not exist")
	} else if strings.Contains(string(responseBody), "Internal server error") {
		log.WithFields(f).Warnf("response contains error: %+v", responseBody)
		return nil, errors.New("internal server error")
	}

	var out requestCorporateSignatureOutput
	err = json.Unmarshal(responseBody, &out)
	if err != nil {
		if _, ok := err.(*json.UnmarshalTypeError); ok {
			return nil, errors.New(string(responseBody))
		}
		return nil, err
	}

	return &out, nil
}

func removeSignatoryRole(ctx context.Context, userEmail string, companySFID string, projectSFID string) error {
	f := logrus.Fields{"functionName": "removeSignatoryRole", "user_email": userEmail, "company_sfid": companySFID, "project_sfid": projectSFID}
	log.WithFields(f).Debug("removing role for user")

	usc := userService.GetClient()
	// search user
	log.WithFields(f).Debug("searching user by email")
	user, err := usc.SearchUserByEmail(userEmail)
	if err != nil {
		log.WithFields(f).Debug("Failed to get user")
		return err
	}

	log.WithFields(f).Debug("Getting role id")
	acsClient := acsService.GetClient()
	roleID, roleErr := acsClient.GetRoleID("cla-signatory")
	if roleErr != nil {
		log.WithFields(f).Debug("Failed to get role id for cla-signatory")
		return roleErr
	}
	// Get scope id
	log.WithFields(f).Debug("getting scope id")
	orgClient := organizationService.GetClient()
	scopeID, scopeErr := orgClient.GetScopeID(ctx, companySFID, projectSFID, "cla-signatory", "project|organization", user.Username)

	if scopeErr != nil {
		log.WithFields(f).Debug("Failed to get scope id for cla-signatory role")
		return scopeErr
	}

	//Unassign role
	log.WithFields(f).Debug("Unassigning role")
	deleteErr := orgClient.DeleteOrgUserRoleOrgScopeProjectOrg(ctx, companySFID, roleID, scopeID, &user.Username, &userEmail)

	if deleteErr != nil {
		log.WithFields(f).Debug("Failed to remove cla-signatory role")
		return deleteErr
	}

	return nil

}

func prepareUserForSigning(ctx context.Context, userEmail string, companySFID, projectSFID, signedEntityName string) error {
	f := logrus.Fields{
		"functionName":     "sign.prepareUserForSigning",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"user_email":       userEmail,
		"company_sfid":     companySFID,
		"project_sfid":     projectSFID,
		"signedEntityName": signedEntityName,
	}

	role := utils.CLASignatoryRole
	log.WithFields(f).Debug("called")
	usc := userService.GetClient()
	// search user
	log.WithFields(f).Debug("searching user by email")
	user, err := usc.SearchUserByEmail(userEmail)
	if err != nil {
		log.WithFields(f).WithError(err).Debugf("User with email: %s does not have an LF login", userEmail)
		return nil
	}

	ac := acsService.GetClient()
	log.WithFields(f).Debugf("getting role_id for %s", role)
	roleID, err := ac.GetRoleID(role)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("getting role_id for %s failed: %v", role, err.Error())
		return err
	}
	log.WithFields(f).Debugf("fetched role %s, role_id %s", role, roleID)
	// assign user role of cla signatory for this project
	osc := organizationService.GetClient()

	// Attempt to assign the cla-signatory role
	log.WithFields(f).Debugf("assigning user role of %s...", role)
	err = osc.CreateOrgUserRoleOrgScopeProjectOrg(ctx, userEmail, projectSFID, companySFID, roleID)
	if err != nil {
		// Log the error - but assigning the cla-signatory role is not a requirement as most users do not have a LF Login - do not throw an error
		if strings.Contains(err.Error(), "associated with some organization") {
			msg := fmt.Sprintf("user: %s already associated with some organization", user.Username)
			log.WithFields(f).WithError(err).Warn(msg)
			// return errors.New(msg)
		} else if _, ok := err.(*organizations.CreateOrgUsrRoleScopesConflict); !ok {
			log.WithFields(f).WithError(err).Warnf("assigning user role of %s failed - user already assigned the role: %v", role, err)
			// return err
		} else {
			log.WithFields(f).WithError(err).Warnf("assigning user role of %s failed: %v", role, err)
		}
	}

	return nil
}
