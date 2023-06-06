// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cla_manager

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/communitybridge/easycla/cla-backend-go/project/repository"
	service2 "github.com/communitybridge/easycla/cla-backend-go/project/service"

	"github.com/go-openapi/strfmt"

	"github.com/sirupsen/logrus"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/communitybridge/easycla/cla-backend-go/emails"
	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/cla_manager"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"
	"github.com/communitybridge/easycla/cla-backend-go/repositories"
	"github.com/communitybridge/easycla/cla-backend-go/v2/organization-service/client/organizations"

	v1ClaManager "github.com/communitybridge/easycla/cla-backend-go/cla_manager"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	v1User "github.com/communitybridge/easycla/cla-backend-go/user"
	easyCLAUser "github.com/communitybridge/easycla/cla-backend-go/users"
	v2AcsService "github.com/communitybridge/easycla/cla-backend-go/v2/acs-service"
	v2Company "github.com/communitybridge/easycla/cla-backend-go/v2/company"
	v2OrgService "github.com/communitybridge/easycla/cla-backend-go/v2/organization-service"
	v2ProjectService "github.com/communitybridge/easycla/cla-backend-go/v2/project-service"
	v2UserService "github.com/communitybridge/easycla/cla-backend-go/v2/user-service"
)

var (
	//ErrCLACompanyNotFound returned if EasyCLA company not found
	ErrCLACompanyNotFound = errors.New("company not found")
	//ErrCLAUserNotFound returned if EasyCLA User is not found
	ErrCLAUserNotFound = errors.New("cla user not found")
	//ErrLFXUserNotFound when user-service fails to find user
	ErrLFXUserNotFound = errors.New("lfx user not found")
	//ErrNoLFID thrown when users dont have an LFID
	ErrNoLFID = errors.New("user has no LF Login")
	//ErrNoOrgAdmins when No admins found for organization
	ErrNoOrgAdmins = errors.New("no admins in company")
	//ErrRoleScopeConflict thrown if user already has role scope
	ErrRoleScopeConflict = errors.New("user is already cla-manager")
	//ErrCLAManagerDesigneeConflict when user is already assigned cla-manager-designee role
	ErrCLAManagerDesigneeConflict = errors.New("user already assigned cla-manager")
	//ErrProjectSigned returns error if project already signed
	ErrProjectSigned = errors.New("project already signed")
	//ErrClaGroupNotFound returns error if cla group not found
	ErrClaGroupNotFound = errors.New("cla group not found")
	//ErrClaGroupBadRequest returns error if cla group bad request
	ErrClaGroupBadRequest = errors.New("cla group bad request")
)

const (
	// used for filtering when fetching contributor email
	excludedNoReplyEmails = "noreply.github.com"
)

type service struct {
	emailTemplateService emails.EmailTemplateService
	companyService       company.IService
	projectService       service2.Service
	repositoriesService  repositories.Service
	managerService       v1ClaManager.IService
	easyCLAUserService   easyCLAUser.Service
	v2CompanyService     v2Company.Service
	eventService         events.Service
	projectCGRepo        projects_cla_groups.Repository
}

// Service interface
type Service interface {
	CreateCLAManager(ctx context.Context, authUser *auth.User, claGroupID string, params cla_manager.CreateCLAManagerParams, authUsername string) (*models.CompanyClaManager, *models.ErrorResponse)
	DeleteCLAManager(ctx context.Context, authUser *auth.User, claGroupID string, params cla_manager.DeleteCLAManagerParams) *models.ErrorResponse
	InviteCompanyAdmin(ctx context.Context, contactAdmin bool, companyID string, projectID string, userEmail string, name string, contributor *v1User.User) ([]*models.ClaManagerDesignee, error)
	CreateCLAManagerDesignee(ctx context.Context, companyID string, projectID string, userEmail string) (*models.ClaManagerDesignee, error)
	CreateCLAManagerRequest(ctx context.Context, contactAdmin bool, companyID string, projectID string, userEmail string, fullName string, authUser *auth.User) (*models.ClaManagerDesignee, error)
	NotifyCLAManagers(ctx context.Context, notifyCLAManagers *models.NotifyClaManagerList, CorporateConsoleV2URL string) error
	CreateCLAManagerDesigneeByGroup(ctx context.Context, params cla_manager.CreateCLAManagerDesigneeByGroupParams, projectCLAGroups []*projects_cla_groups.ProjectClaGroup) ([]*models.ClaManagerDesignee, string, error)
	ProjectCompanySignedOrNot(ctx context.Context, signedAtFoundation bool, projectCLAGroups []*projects_cla_groups.ProjectClaGroup, companyModel *v1Models.Company) error
	IsCLAManagerDesignee(ctx context.Context, companySFID, claGroupID, userLFID string) (*models.UserRoleStatus, error)

	// Email Functions
	SendEmailToCLAManager(ctx context.Context, input *EmailToCLAManagerModel, projectSFIDs []string)
	SendEmailToOrgAdmin(ctx context.Context, input EmailToOrgAdminModel)
	ContributorEmailToOrgAdmin(ctx context.Context, input ContributorEmailToOrgAdminModel)
	SendEmailToCLAManagerDesigneeCorporate(ctx context.Context, input ToCLAManagerDesigneeCorporateModel)
	SendEmailToCLAManagerDesignee(ctx context.Context, input ToCLAManagerDesigneeModel)
	SendDesigneeEmailToUserWithNoLFID(ctx context.Context, input DesigneeEmailToUserWithNoLFIDModel) error
	SendEmailToUserWithNoLFID(ctx context.Context, input EmailToUserWithNoLFIDModel) error
}

// NewService returns instance of CLA Manager service
func NewService(emailTemplateService emails.EmailTemplateService, compService company.IService, projService service2.Service, mgrService v1ClaManager.IService, claUserService easyCLAUser.Service,
	repoService repositories.Service, v2CompService v2Company.Service,
	evService events.Service, projectCGroupRepo projects_cla_groups.Repository) Service {
	return &service{
		emailTemplateService: emailTemplateService,
		companyService:       compService,
		projectService:       projService,
		repositoriesService:  repoService,
		managerService:       mgrService,
		easyCLAUserService:   claUserService,
		v2CompanyService:     v2CompService,
		eventService:         evService,
		projectCGRepo:        projectCGroupRepo,
	}
}

// CreateCLAManager creates Cla Manager
func (s *service) CreateCLAManager(ctx context.Context, authUser *auth.User, claGroupID string, params cla_manager.CreateCLAManagerParams, authUsername string) (*models.CompanyClaManager, *models.ErrorResponse) {
	f := logrus.Fields{
		"functionName":   "cla_manager.service.CreateCLAManager",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"projectSFID":    params.ProjectSFID,
		"companyID":      params.CompanyID,
		"authUsername":   authUsername,
		"xUserName":      params.XUSERNAME,
		"xEmail":         params.XEMAIL,
	}

	// Search for Company by internal ID
	log.WithFields(f).Debugf("Getting company by ID: %s", params.CompanyID)
	v1CompanyModel, companyErr := s.companyService.GetCompany(ctx, params.CompanyID)
	if companyErr != nil || v1CompanyModel == nil {
		msg := buildErrorMessage("company lookup error", claGroupID, params, companyErr)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}
	f["companySFID"] = v1CompanyModel.CompanyExternalID
	f["companyName"] = v1CompanyModel.CompanyName

	claGroup, err := s.projectService.GetCLAGroupByID(ctx, claGroupID)
	if err != nil || claGroup == nil {
		msg := buildErrorMessage("cla group search by ID failure", claGroupID, params, err)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	// Get user by email
	userServiceClient := v2UserService.GetClient()
	// Get Manager lf account by username. Used for email content
	managerUser, mgrErr := userServiceClient.GetUserByUsername(authUsername)
	if mgrErr != nil || managerUser == nil {
		msg := fmt.Sprintf("Failed to get Lfx User with username : %s ", authUsername)
		log.WithFields(f).Warn(msg)
	}
	user, userErr := userServiceClient.SearchUserByEmail(params.Body.UserEmail.String())

	// Check for potential user with no username
	if user != nil && user.Username == "" {
		msg := fmt.Sprintf("User %s needs to update account with username", params.Body.UserEmail.String())
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	if userErr != nil {
		msg := fmt.Sprintf("User %s has no LF Login account. User can be added as CLA Manager after LF Login is created", params.Body.UserEmail.String())
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	// Check if user exists in easyCLA DB, if not add User
	log.WithFields(f).Debugf("Checking user: %+v in EasyCLA database...", user)
	claUser, claUserErr := s.easyCLAUserService.GetUserByLFUserName(user.Username)
	if claUserErr != nil {
		msg := fmt.Sprintf("Problem getting claUser by :%s, error: %+v ", user.Username, claUserErr)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	if claUser == nil {
		msg := fmt.Sprintf("User not found when searching by LF Login: %s and shall be created", user.Username)
		log.WithFields(f).Debug(msg)
		userName := fmt.Sprintf("%s %s", *params.Body.FirstName, *params.Body.LastName)
		_, currentTimeString := utils.CurrentTime()
		claUserModel := &v1Models.User{
			CompanyID:      v1CompanyModel.CompanyID,
			UserExternalID: v1CompanyModel.CompanyExternalID,
			LfEmail:        strfmt.Email(*user.Emails[0].EmailAddress),
			Admin:          true,
			LfUsername:     user.Username,
			DateCreated:    currentTimeString,
			DateModified:   currentTimeString,
			Username:       userName,
			Version:        "v1",
		}
		newUserModel, userModelErr := s.easyCLAUserService.CreateUser(claUserModel, nil)
		if userModelErr != nil {
			msg := fmt.Sprintf("Failed to create user : %+v", claUserModel)
			log.WithFields(f).Warn(msg)
			return nil, &models.ErrorResponse{
				Message: msg,
				Code:    "400",
			}
		}
		log.WithFields(f).Debugf("Created easyCLAUser %+v ", newUserModel)
	}

	// GetSFProject
	ps := v2ProjectService.GetClient()
	projectSF, projectErr := ps.GetProject(params.ProjectSFID)
	if projectErr != nil {
		msg := buildErrorMessage("project service lookup error", claGroupID, params, projectErr)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	// Add CLA Manager to Database
	signature, addErr := s.managerService.AddClaManager(ctx, authUser, v1CompanyModel.CompanyID, claGroupID, user.Username, projectSF.Name)
	if addErr != nil {
		msg := buildErrorMessageCreate(params, addErr)
		log.WithFields(f).Warn(msg)
		return nil, &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}
	if signature == nil {
		sigMsg := fmt.Sprintf("Signature not found for project: %s and company: %s ", claGroupID, v1CompanyModel.CompanyID)
		log.WithFields(f).Warn(sigMsg)
		return nil, &models.ErrorResponse{
			Message: sigMsg,
			Code:    "400",
		}
	}

	claCompanyManager := &models.CompanyClaManager{
		LfUsername:       user.Username,
		Email:            *params.Body.UserEmail,
		UserSfid:         user.ID,
		ApprovedOn:       time.Now().String(),
		ProjectSfid:      params.ProjectSFID,
		ClaGroupName:     claGroup.ProjectName,
		ProjectID:        claGroupID,
		ProjectName:      projectSF.Name,
		OrganizationName: v1CompanyModel.CompanyName,
		OrganizationSfid: v1CompanyModel.CompanyExternalID,
		OrganizationID:   v1CompanyModel.CompanyID,
		Name:             fmt.Sprintf("%s %s", user.FirstName, user.LastName),
	}

	return claCompanyManager, nil
}

func (s *service) DeleteCLAManager(ctx context.Context, authUser *auth.User, claGroupID string, params cla_manager.DeleteCLAManagerParams) *models.ErrorResponse {
	f := logrus.Fields{
		"functionName":   "cla_manager.service.DeleteCLAManager",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectSFID":    params.ProjectSFID,
		"companyID":      params.CompanyID,
		"authUserName":   authUser.UserName,
		"authUserEmail":  authUser.Email,
	}

	signature, deleteErr := s.managerService.RemoveClaManager(ctx, authUser, params.CompanyID, claGroupID, params.UserLFID)

	if deleteErr != nil {
		msg := buildErrorMessageDelete(params, deleteErr)
		log.WithFields(f).Warn(msg)
		return &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	if signature == nil {
		msg := fmt.Sprintf("CCLA signature not found for project: %s and company: %s ", claGroupID, params.CompanyID)
		log.WithFields(f).Warn(msg)
		return &models.ErrorResponse{
			Message: msg,
			Code:    "400",
		}
	}

	return nil
}

// CreateCLAManagerDesignee creates designee for cla manager prospect
func (s *service) CreateCLAManagerDesignee(ctx context.Context, companyID string, projectSFID string, userEmail string) (*models.ClaManagerDesignee, error) {
	f := logrus.Fields{
		"functionName":   "cla_manager.service.CreateCLAManagerDesignee",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"projectSFID":    projectSFID,
		"userEmail":      userEmail,
	}

	// integrate user,acs,org and project services
	acServiceClient := v2AcsService.GetClient()
	orgClient := v2OrgService.GetClient()
	projectClient := v2ProjectService.GetClient()

	log.WithFields(f).Debugf("loading company by ID...")
	v1CompanyModel, companyErr := s.companyService.GetCompany(ctx, companyID)
	if companyErr != nil {
		log.WithFields(f).Warnf("company not found, error: %+v", companyErr)
		return nil, companyErr
	}
	f["companySFID"] = v1CompanyModel.CompanyExternalID
	f["companyName"] = v1CompanyModel.CompanyName

	log.WithFields(f).Debugf("checking if company/project is signed with CLA managers...")
	isSigned, signedErr := s.isSigned(ctx, v1CompanyModel, projectSFID)
	if signedErr != nil {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request - %s", signedErr)
		log.WithFields(f).Warn(msg)
		return nil, signedErr
	}

	if isSigned {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request - Project: %s is already signed", projectSFID)
		log.WithFields(f).Warn(msg)
		return nil, ErrProjectSigned
	}

	userService := v2UserService.GetClient()
	log.WithFields(f).Debug("searching user in user service...")
	// This routine is taking 24-29 seconds when running locally -> User service in DEV
	//lfxUser, userErr := userService.SearchUserByEmail(userEmail)
	// This routine is taking 4 seconds when running locally -> User service in DEV
	lfxUser, userErr := userService.SearchUsersByEmail(userEmail)
	if userErr != nil {
		log.WithFields(f).Debugf("Failed to get user by email: %s, error: %+v", userEmail, userErr)
		return nil, ErrLFXUserNotFound
	}

	log.WithFields(f).Debugf("user found: %+v", lfxUser)

	log.WithFields(f).Debugf("checking if user has %s role scope...", utils.CLADesigneeRole)
	// Check if user is already CLA Manager designee of project|organization scope
	hasRoleScope, hasRoleScopeErr := orgClient.IsUserHaveRoleScope(ctx, utils.CLADesigneeRole, lfxUser.ID, v1CompanyModel.CompanyExternalID, projectSFID)
	if hasRoleScopeErr != nil {
		// Skip 404 for ListOrgUsrServiceScopes endpoint
		if _, ok := hasRoleScopeErr.(*organizations.ListOrgUsrServiceScopesNotFound); !ok {
			log.WithFields(f).Debugf("Failed to check roleScope: %s for user: %s", utils.CLADesigneeRole, lfxUser.Username)
			return nil, hasRoleScopeErr
		}
	}
	if hasRoleScope {
		// this is fine...just return with value...
		log.WithFields(f).Debugf("User already has role scope: %s", utils.CLADesigneeRole)
	}

	log.WithFields(f).Debug("loading project by SFID...")
	projectSF, projectErr := projectClient.GetProject(projectSFID)
	if projectErr != nil {
		log.WithFields(f).Debugf("problem getting project: %s from the project service, error: %+v", projectSFID, projectErr)
		return nil, projectErr
	}

	log.WithFields(f).Debugf("loading role ID for %s...", utils.CLADesigneeRole)
	roleID, designeeErr := acServiceClient.GetRoleID(utils.CLADesigneeRole)
	if designeeErr != nil {
		log.WithFields(f).Warnf("Problem getting role ID for cla-manager-designee, error: %+v", designeeErr)
		return nil, designeeErr
	}

	log.WithFields(f).Debugf("creating user role organization scope for user: %s, with role: %s with role ID: %s using project|org: %s|%s...",
		userEmail, utils.CLADesigneeRole, roleID, projectSFID, v1CompanyModel.CompanyExternalID)
	scopeErr := orgClient.CreateOrgUserRoleOrgScopeProjectOrg(ctx, userEmail, projectSFID, v1CompanyModel.CompanyExternalID, roleID)
	if scopeErr != nil {
		// Ignore conflict - role has already been assigned - otherwise, return error
		if _, ok := scopeErr.(*organizations.CreateOrgUsrRoleScopesConflict); !ok {
			log.Warn(fmt.Sprintf("problem creating projectOrg scope for email: %s , projectSFID: %s, companySFID: %s", userEmail, projectSFID, v1CompanyModel.CompanyExternalID))
			return nil, scopeErr
		}
	}

	log.WithFields(f).Debugf("created user role organization scope for user: %s, with role: %s with role ID: %s using project|org: %s|%s...",
		userEmail, utils.CLADesigneeRole, roleID, projectSFID, v1CompanyModel.CompanyExternalID)

	// Log Event
	s.eventService.LogEventWithContext(ctx,
		&events.LogEventArgs{
			EventType:    events.AssignUserRoleScopeType,
			ProjectSFID:  projectSFID,
			CompanyModel: v1CompanyModel,
			CompanyID:    v1CompanyModel.CompanyID,
			UserName:     lfxUser.Username,
			LfUsername:   lfxUser.Username,
			EventData: &events.AssignRoleScopeData{
				Role:      "cla-manager-designee",
				Scope:     fmt.Sprintf("%s|%s", projectSFID, v1CompanyModel.CompanyExternalID),
				UserName:  lfxUser.Username,
				UserEmail: userEmail,
			},
		})

	claManagerDesignee := &models.ClaManagerDesignee{
		LfUsername:  lfxUser.Username,
		UserSfid:    lfxUser.ID,
		Type:        lfxUser.Type,
		AssignedOn:  time.Now().String(),
		Email:       strfmt.Email(userEmail),
		ProjectSfid: projectSFID,
		CompanySfid: v1CompanyModel.CompanyExternalID,
		CompanyID:   v1CompanyModel.CompanyID,
		ProjectName: projectSF.Name,
	}

	return claManagerDesignee, nil
}

func (s *service) IsCLAManagerDesignee(ctx context.Context, companySFID, claGroupID, userLFID string) (*models.UserRoleStatus, error) {
	f := logrus.Fields{
		"functionName":   "cla_manager.service.IsCLAManagerDesignee",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companySFID":    companySFID,
		"claGroupID":     claGroupID,
		"userLFID":       userLFID,
	}

	// Get LF User
	userClient := v2UserService.GetClient()
	user, userErr := userClient.GetUserByUsername(userLFID)
	if userErr != nil {
		log.WithFields(f).Warnf("Failed to get user by username: %s , error: %+v", userLFID, userErr)
		return nil, userErr
	}

	log.WithFields(f).Debugf("Getting project sf mappings for claGroupID: %s ", claGroupID)
	pcgs, pcgErr := s.projectCGRepo.GetProjectsIdsForClaGroup(ctx, claGroupID)
	if pcgErr != nil {
		log.WithFields(f).Warnf("Problem getting mappings for claGroup: %s , error: %+v ", claGroupID, pcgErr)
		return nil, pcgErr
	}

	orgClient := v2OrgService.GetClient()
	//default to false
	var hasRole = false

	if len(pcgs) > 0 {
		// Check for role at project level
		log.WithFields(f).Debugf("Checking role for user: %s at project level for %d projects", user.ID, len(pcgs))
		type result struct {
			hasRole     bool
			projectSFID string
			err         error
		}
		roleStatusChan := make(chan *result)
		var wg sync.WaitGroup
		wg.Add(len(pcgs))

		go func() {
			wg.Wait()
			close(roleStatusChan)
		}()

		for _, pcg := range pcgs {
			go func(swg *sync.WaitGroup, pcg *projects_cla_groups.ProjectClaGroup, roleStatusChan chan *result) {
				defer swg.Done()
				var output result
				log.WithFields(f).Debugf("Checking role status for projectSFID: %s", pcg.ProjectSFID)
				hasProjectLevelRole, roleErr := orgClient.IsUserHaveRoleScope(ctx, utils.CLADesigneeRole, user.ID, companySFID, pcg.ProjectSFID)
				if roleErr != nil {
					log.WithFields(f).Debugf("problem getting role:%s for user and project: %s ", utils.CLADesigneeRole, pcg.ProjectSFID)
					output = result{
						hasRole:     false,
						err:         roleErr,
						projectSFID: pcg.ProjectSFID,
					}
					roleStatusChan <- &output
					return
				}
				if hasProjectLevelRole {
					log.WithFields(f).Debugf("user has :%s role for company: %s ", utils.CLADesigneeRole, companySFID)
					roleStatusChan <- &result{
						hasRole:     true,
						err:         nil,
						projectSFID: pcg.ProjectSFID,
					}
				} else {
					log.WithFields(f).Debugf("user does not have :%s role for company: %s ", utils.CLADesigneeRole, companySFID)
					roleStatusChan <- &result{
						hasRole:     false,
						err:         nil,
						projectSFID: pcg.ProjectSFID,
					}
				}

			}(&wg, pcg, roleStatusChan)
		}

		//confirm user has cla-manager-designee for any of the  projects
		for resultCh := range roleStatusChan {
			if resultCh.err != nil {
				return nil, resultCh.err
			}

			if resultCh.hasRole {
				log.WithFields(f).Debugf("User %s has %s role for project : %s", userLFID, utils.CLADesigneeRole, resultCh.projectSFID)
				hasRole = true
				return &models.UserRoleStatus{
					HasRole:    &hasRole,
					LfUsername: userLFID,
				}, nil
			}
		}
		log.WithFields(f).Debugf("User %s has %s role at project level", userLFID, utils.CLADesigneeRole)
		hasRole = true

	}

	return &models.UserRoleStatus{
		HasRole:    &hasRole,
		LfUsername: userLFID,
	}, nil
}

// CreateCLAManagerDesigneeByGroup creates designee by group for cla manager prospect
func (s *service) CreateCLAManagerDesigneeByGroup(ctx context.Context, params cla_manager.CreateCLAManagerDesigneeByGroupParams, projectCLAGroups []*projects_cla_groups.ProjectClaGroup) ([]*models.ClaManagerDesignee, string, error) {
	f := logrus.Fields{
		"functionName":   "cla_manager.service.CreateCLAManagerDesigneeByGroup",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     params.ClaGroupID,
		"companyID":      params.CompanyID,
		"userEmail":      params.Body.UserEmail.String(),
	}

	var designeeScopes []*models.ClaManagerDesignee
	userEmail := params.Body.UserEmail.String()

	// Lookup the company by internal ID
	log.WithFields(f).Debugf("looking up company by internal ID...")
	v1CompanyModel, err := s.companyService.GetCompany(ctx, params.CompanyID)
	if err != nil || v1CompanyModel == nil {
		msg := fmt.Sprintf("unable to lookup company by ID: %s", params.CompanyID)
		log.WithFields(f).WithError(err).Warn(msg)
		return nil, msg, err
	}
	f["companySFID"] = v1CompanyModel.CompanyExternalID
	f["companyName"] = v1CompanyModel.CompanyName

	// Channel result
	type result struct {
		designee *models.ClaManagerDesignee
		msg      string
		err      error
	}
	designeeChan := make(chan *result)
	var wg sync.WaitGroup
	wg.Add(len(projectCLAGroups))

	go func() {
		wg.Wait()
		close(designeeChan)
	}()

	for _, pcg := range projectCLAGroups {
		go func(swg *sync.WaitGroup, pcg *projects_cla_groups.ProjectClaGroup, designeeChannel chan *result) {
			defer swg.Done()
			log.WithFields(f).Debugf("creating CLA Manager Designee for Project SFID: %s", pcg.ProjectSFID)
			claManagerDesignee, err := s.CreateCLAManagerDesignee(ctx, v1CompanyModel.CompanyID, pcg.ProjectSFID, userEmail)
			var output result
			if err != nil {
				if err == ErrCLAManagerDesigneeConflict {
					msg := fmt.Sprintf("Conflict assigning cla manager role for Project SFID: %s, error: %s ", pcg.ProjectSFID, err)
					output = result{
						designee: nil,
						msg:      msg,
						err:      ErrCLAManagerDesigneeConflict,
					}
				}
				msg := fmt.Sprintf("Creating cla manager failed for Project SFID: %s, error: %s ", pcg.ProjectSFID, err)
				output = result{
					designee: nil,
					msg:      msg,
					err:      err,
				}
				designeeChannel <- &output
				return
			}
			output = result{
				designee: claManagerDesignee,
				msg:      "",
				err:      nil,
			}
			designeeChannel <- &output
		}(&wg, pcg, designeeChan)
	}
	for resultCh := range designeeChan {
		if resultCh.err != nil {
			return nil, resultCh.msg, resultCh.err
		}
		designeeScopes = append(designeeScopes, resultCh.designee)
	}

	return designeeScopes, "", nil
}

// CreateCLAManagerRequest service method
func (s *service) CreateCLAManagerRequest(ctx context.Context, contactAdmin bool, companyID string, projectID string, userEmail string, fullName string, authUser *auth.User) (*models.ClaManagerDesignee, error) {
	f := logrus.Fields{
		"functionName":   "cla_manager.service.CreateCLAManagerRequest",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"contactAdmin":   contactAdmin,
		"companyID":      companyID,
		"projectID":      projectID,
		"userEmail":      userEmail,
		"fullName":       fullName,
		"authUserName":   authUser.UserName,
		"authUserEmail":  authUser.Email,
	}

	orgService := v2OrgService.GetClient()
	userService := v2UserService.GetClient()

	log.WithFields(f).Debugf("loading company by external ID...")
	// Search for salesForce Company aka external Company
	v1CompanyModel, companyErr := s.companyService.GetCompany(ctx, companyID)
	if companyErr != nil {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request - %s", companyErr)
		log.Warn(msg)
		return nil, companyErr
	}
	f["companyID"] = v1CompanyModel.CompanyID
	f["companyName"] = v1CompanyModel.CompanyName

	// Determine if the CCLA is already signed or not
	log.WithFields(f).Debugf("checking if company/project is signed with CLA managers...")
	isSigned, signedErr := s.isSigned(ctx, v1CompanyModel, projectID)
	if signedErr != nil {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request - %s", signedErr)
		log.WithFields(f).Warn(msg)
		return nil, signedErr
	}

	if isSigned {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request - Project: %s is already signed ", projectID)
		log.WithFields(f).Warn(msg)
		return nil, ErrProjectSigned
	}

	log.WithFields(f).Debugf("querying project service for project details...")
	// GetSFProject
	ps := v2ProjectService.GetClient()
	projectSF, projectErr := ps.GetProject(projectID)
	if projectErr != nil {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request - Project service lookup error for SFID: %s, error : %+v",
			projectID, projectErr)
		log.WithFields(f).Warn(msg)
		return nil, projectErr
	}

	// Check if sending cla manager request to company admin
	if contactAdmin {
		log.WithFields(f).Debug("sending email to company Admin")
		log.WithFields(f).Debug("querying user admin scopes...")
		scopes, listScopeErr := orgService.ListOrgUserAdminScopes(ctx, v1CompanyModel.CompanyExternalID, nil)
		if listScopeErr != nil {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - Admin lookup error for organisation SFID: %s, error: %+v ",
				v1CompanyModel.CompanyExternalID, listScopeErr)
			log.WithFields(f).Warn(msg)
			return nil, listScopeErr
		}

		if len(scopes.Userroles) == 0 {
			msg := fmt.Sprintf("EasyCLA - 404 NotFound - No admins for organization SFID: %s",
				v1CompanyModel.CompanyExternalID)
			log.WithFields(f).Warn(msg)
			return nil, ErrNoOrgAdmins
		}

		for _, admin := range scopes.Userroles {
			log.WithFields(f).Debugf("sending email to organization admin: %+v", admin)

			adminUser, adminErr := userService.GetUser(admin.Contact.ID)
			if adminErr != nil {
				msg := fmt.Sprintf("Failed to get user for ID: %s ", admin.Contact.ID)
				log.Warn(msg)
				return nil, adminErr
			}
			s.SendEmailToOrgAdmin(ctx,
				EmailToOrgAdminModel{
					adminEmail:  userService.GetPrimaryEmail(adminUser),
					adminName:   admin.Contact.Name,
					companyName: v1CompanyModel.CompanyName,
					projectName: projectSF.Name,
					projectSFID: projectSF.ID,
					senderName:  authUser.UserName,
					senderEmail: authUser.Email,
				})
			// Make a note in the event log
			s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
				EventType:   events.ContributorNotifyCompanyAdminType,
				LfUsername:  authUser.UserName,
				ProjectSFID: projectID,
				CompanyID:   v1CompanyModel.CompanyID,
				EventData: &events.ContributorNotifyCompanyAdminData{
					AdminName:  admin.Contact.Name,
					AdminEmail: userService.GetPrimaryEmail(adminUser),
				},
			})
		}

		return nil, nil
	}
	log.WithFields(f).Debug("not sending admin email...")

	log.WithFields(f).Debug("searching user in user service...")
	// This routine is taking 24-29 seconds when running locally -> User service in DEV
	//lfxUser, userErr := userService.SearchUserByEmail(userEmail)
	// This routine is taking 4 seconds when running locally -> User service in DEV
	lfxUser, userErr := userService.SearchUsersByEmail(userEmail)
	if userErr != nil {
		msg := fmt.Sprintf("User: %s does not have an LF Login", userEmail)
		log.WithFields(f).Warn(msg)
		// Send email
		sendEmailErr := s.SendEmailToUserWithNoLFID(ctx, EmailToUserWithNoLFIDModel{
			projectName:         projectSF.Name,
			requesterUsername:   authUser.UserName,
			requesterEmail:      authUser.Email,
			userWithNoLFIDName:  fullName,
			userWithNoLFIDEmail: userEmail,
			organizationID:      v1CompanyModel.CompanyExternalID,
			companyName:         v1CompanyModel.CompanyName,
			projectID:           projectSF.ID,
			role:                utils.CLADesigneeRole,
		})
		if sendEmailErr != nil {
			log.WithFields(f).Warnf("Error sending email: %+v", sendEmailErr)
			return nil, sendEmailErr
		}
		return nil, ErrNoLFID
	}

	if lfxUser.Username == "" {
		return nil, ErrNoLFID
	}

	log.WithFields(f).Debug("sending CLA manager designee request...")
	claManagerDesignee, err := s.CreateCLAManagerDesignee(ctx, companyID, projectID, userEmail)
	if err != nil {
		// Check conflict for role scope
		if _, ok := err.(*organizations.CreateOrgUsrRoleScopesConflict); ok {
			log.WithFields(f).Warn("problem creating organization role scope for designee - role exists")
			return nil, ErrRoleScopeConflict
		}
		log.WithFields(f).Warnf("problem creating organization role scope for designee, error: %+v", err)
		return nil, err
	}

	log.WithFields(f).Debug("creating a contributor assigned CLA designee log event...")
	// Make a note in the event log
	s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType:   events.ContributorAssignCLADesigneeType,
		LfUsername:  authUser.UserName,
		ProjectSFID: projectID,
		CompanyID:   v1CompanyModel.CompanyID,
		EventData: &events.ContributorAssignCLADesignee{
			DesigneeName:  claManagerDesignee.LfUsername,
			DesigneeEmail: claManagerDesignee.Email.String(),
		},
	})

	log.WithFields(f).Debugf("sending Email to CLA Manager Designee email: %s ", userEmail)
	designeeName := fmt.Sprintf("%s %s", lfxUser.FirstName, lfxUser.LastName)
	s.SendEmailToCLAManagerDesigneeCorporate(ctx, ToCLAManagerDesigneeCorporateModel{
		companyName:   v1CompanyModel.CompanyName,
		projectSFID:   projectSF.ID,
		projectName:   projectSF.Name,
		designeeEmail: userEmail,
		designeeName:  designeeName,
		senderEmail:   authUser.Email,
		senderName:    authUser.UserName,
	})

	log.WithFields(f).Debug("creating a contributor notify CLA designee log event...")
	// Make a note in the event log
	s.eventService.LogEventWithContext(ctx, &events.LogEventArgs{
		EventType:   events.ContributorNotifyCLADesigneeType,
		LfUsername:  authUser.UserName,
		ProjectSFID: projectID,
		CompanyID:   v1CompanyModel.CompanyID,
		EventData: &events.ContributorNotifyCLADesignee{
			DesigneeName:  claManagerDesignee.LfUsername,
			DesigneeEmail: claManagerDesignee.Email.String(),
		},
	})

	log.WithFields(f).Debugf("CLA Manager designee created: %+v", claManagerDesignee)
	return claManagerDesignee, nil
}

func (s *service) ValidateInviteCompanyAdminCheck(ctx context.Context, f logrus.Fields, projectID string, contactAdmin bool, userEmail string, name string, contributor *v1User.User) error {
	validateError := validateInviteCompanyAdmin(contactAdmin, userEmail, name, contributor)
	if validateError != nil {
		return validateError
	}

	claGroupModel, projectErr := s.projectService.GetCLAGroupByID(ctx, projectID)
	if projectErr != nil || claGroupModel == nil {
		log.WithFields(f).WithError(projectErr).Warn("problem loading CLA group by ID")

		var e *utils.CLAGroupNotFound
		if errors.As(projectErr, &e) {
			log.WithFields(f).WithError(projectErr).Warn("problem loading CLA group by ID - cla group not found")
			return ErrClaGroupNotFound

		}
		if errors.Is(projectErr, repository.ErrProjectDoesNotExist) {
			log.WithFields(f).WithError(projectErr).Warn("problem cla group not found")
			return ErrClaGroupNotFound
		}
		return ErrClaGroupBadRequest

	}
	return nil
}

func (s *service) InviteCompanyAdmin(ctx context.Context, contactAdmin bool, companyID string, projectID string, userEmail string, name string, contributor *v1User.User) ([]*models.ClaManagerDesignee, error) { //nolint
	f := logrus.Fields{
		"functionName":   "cla_manager.service.InviteCompanyAdmin",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyID,
		"claGroupID":     projectID,
		"userEmail":      userEmail,
		"name":           name,
	}

	orgService := v2OrgService.GetClient()
	projectService := v2ProjectService.GetClient()
	userService := v2UserService.GetClient()

	validateError := s.ValidateInviteCompanyAdminCheck(ctx, f, projectID, contactAdmin, userEmail, name, contributor)
	if validateError != nil {
		return nil, validateError
	}

	// Get project cla Group records
	log.WithFields(f).Debugf("Getting SalesForce Projects for claGroup: %s ", projectID)
	projectCLAGroups, getErr := s.projectCGRepo.GetProjectsIdsForClaGroup(ctx, projectID)
	if getErr != nil {
		msg := fmt.Sprintf("Error getting SF projects for claGroup: %s ", projectID)
		log.Debug(msg)
	}

	if len(projectCLAGroups) == 0 {
		msg := fmt.Sprintf("Error getting SF projects for claGroup: %s ", projectID)
		return nil, errors.New(msg)
	}

	signedAtFoundation, signedErr := s.projectService.SignedAtFoundationLevel(ctx, projectCLAGroups[0].FoundationSFID)
	if signedErr != nil {
		msg := fmt.Sprintf("Problem checking project: %s , error: %+v", projectID, signedErr)
		log.WithFields(f).Warn(msg)
		return nil, signedErr
	}

	// Get company
	log.WithFields(f).Debugf("Get company for companyID: %s ", companyID)
	companyModel, companyErr := s.companyService.GetCompany(ctx, companyID)
	if companyErr != nil {
		msg := fmt.Sprintf("Problem getting company for companyID: %s ", companyID)
		log.Warn(msg)
		log.Error("company error ", companyErr)
		if companyErr.Error() == "company does not exist" {
			return nil, ErrCLACompanyNotFound
		}
		return nil, companyErr
	}

	if companyModel.CompanyExternalID == "" {
		return nil, ErrCLACompanyNotFound
	}

	organization, orgErr := orgService.GetOrganization(ctx, companyModel.CompanyExternalID)
	if orgErr != nil {
		msg := fmt.Sprintf("Problem getting company by ID: %s ", companyID)
		log.Warn(msg)
		return nil, orgErr
	}

	var projectSFs []string
	var projectSFIDs []string
	foundationSFID := projectCLAGroups[0].FoundationSFID

	if signedAtFoundation {

		// Get salesforce project by FoundationID
		log.WithFields(f).Debugf("querying project service for project details...")
		// GetSFProject
		foundationSF, projectErr := projectService.GetProject(foundationSFID)
		if projectErr != nil {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - Project service lookup error for SFID: %s, error : %+v",
				projectID, projectErr)
			log.WithFields(f).Warn(msg)
			return nil, projectErr
		}
		projectSFs = append(projectSFs, foundationSF.Name)
		projectSFIDs = append(projectSFIDs, foundationSFID)
	} else {
		for _, pcg := range projectCLAGroups {
			log.WithFields(f).Debugf("Getting salesforce project by SFID: %s ", pcg.ProjectSFID)
			projectSF, projectErr := projectService.GetProject(pcg.ProjectSFID)
			if projectErr != nil {
				msg := fmt.Sprintf("Problem getting salesforce Project ID: %s", pcg.ProjectSFID)
				log.WithFields(f).Warn(msg)
				return nil, projectErr
			}
			projectSFs = append(projectSFs, projectSF.Name)
			projectSFIDs = append(projectSFIDs, projectSF.ID)
		}
	}

	var designeeScopes []*models.ClaManagerDesignee

	// Check if sending cla manager request to company admin
	if contactAdmin {
		log.Debugf("Sending email to company Admin")
		scopes, listScopeErr := orgService.ListOrgUserAdminScopes(ctx, companyModel.CompanyExternalID, nil)
		if listScopeErr != nil {
			msg := fmt.Sprintf("Admin lookup error for organisation SFID: %s ", companyModel.CompanyExternalID)
			log.WithFields(f).Warn(msg)
			return nil, listScopeErr
		}
		// Search for Easy CLA User
		log.Debugf("Getting user by ID: %s", contributor.UserID)
		userModel, userErr := s.easyCLAUserService.GetUser(contributor.UserID)
		if userErr != nil {
			msg := fmt.Sprintf("Problem getting user by ID: %s ", contributor.UserID)
			log.Warn(msg)
			return nil, userErr
		}

		for _, admin := range scopes.Userroles {
			// Email details are masked so an extra query to get user details is used
			log.WithFields(f).Debugf("Getting email for user with ID: %s ", admin.Contact.ID)

			adminUser, adminErr := userService.GetUser(admin.Contact.ID)
			if adminErr != nil {
				msg := fmt.Sprintf("Failed to get user for ID: %s ", admin.Contact.ID)
				log.Warn(msg)
				return nil, adminErr
			}

			s.ContributorEmailToOrgAdmin(ctx, ContributorEmailToOrgAdminModel{
				adminEmail:   userService.GetPrimaryEmail(adminUser),
				adminName:    admin.Contact.Name,
				companyName:  organization.Name,
				projectSFIDs: projectSFIDs,
				contributor:  userModel,
				userDetails:  getFormattedUserDetails(userModel),
			})
			designeeScope := models.ClaManagerDesignee{
				Email: strfmt.Email(admin.Contact.EmailAddress),
				Name:  admin.Contact.Name,
			}
			designeeScopes = append(designeeScopes, &designeeScope)
		}
		return designeeScopes, nil
	}

	signedError := s.ProjectCompanySignedOrNot(ctx, signedAtFoundation, projectCLAGroups, companyModel)
	if signedError != nil {
		return nil, signedError
	}

	// Get suggested CLA Manager user details
	user, userErr := userService.SearchUserByEmail(userEmail)
	if userErr != nil || (user != nil && user.Username == "") {
		var contributorModel emails.Contributor
		msg := fmt.Sprintf("UserEmail: %s has no LF Login and has been sent an invite email to create an account , error: %+v", userEmail, userErr)
		log.Warn(msg)

		// Get username and useremail details for contributor
		if contributor.LFEmail != "" && contributor.UserName != "" {
			contributorModel.Email = contributor.LFEmail
			contributorModel.Username = contributor.LFUsername
			contributorModel.EmailLabel = utils.EmailLabel
			contributorModel.UsernameLabel = utils.UserLabel
		} else {
			contributorModel = getContributorPublicEmail(contributor)
		}

		sendErr := s.SendDesigneeEmailToUserWithNoLFID(ctx, DesigneeEmailToUserWithNoLFIDModel{
			userWithNoLFIDName:  name,
			userWithNoLFIDEmail: userEmail,
			contributorModel:    contributorModel,
			projectNames:        projectSFs,
			projectSFIDs:        projectSFIDs,
			foundationSFID:      foundationSFID,
			role:                "cla-manager-designee",
			companyName:         organization.Name,
			organizationID:      organization.ID,
		})
		if sendErr != nil {
			msg := fmt.Sprintf("Problem sending email to user: %s , error: %+v", userEmail, sendErr)
			log.Warn(msg)
		}

		return nil, ErrNoLFID
	}

	if signedAtFoundation {
		// check if claGroup is signed at foundation level
		foundationSFID := projectCLAGroups[0].FoundationSFID
		log.WithFields(f).Debugf("Create cla manager designee for foundation : %s ", foundationSFID)
		claManagerDesignee, err := s.CreateCLAManagerDesignee(ctx, companyID, foundationSFID, userEmail)
		if err != nil {
			msg := fmt.Sprintf("Problem creating cla Manager Designee for user : %s, error: %+v ", userEmail, err)
			log.WithFields(f).Warn(msg)
			return nil, err
		}
		designeeScopes = append(designeeScopes, claManagerDesignee)
	} else {
		for _, pcg := range projectCLAGroups {
			log.WithFields(f).Debugf("Create cla manager designee for Project SFID: %s", pcg.ProjectSFID)
			claManagerDesignee, err := s.CreateCLAManagerDesignee(ctx, companyID, pcg.ProjectSFID, userEmail)
			if err != nil {
				msg := fmt.Sprintf("Problem creating cla Manager Designee for user : %s, error: %+v ", userEmail, err)
				log.WithFields(f).Warn(msg)
				return nil, err
			}
			designeeScopes = append(designeeScopes, claManagerDesignee)
		}
	}

	log.Debugf("Sending Email to CLA Manager Designee email: %s ", userEmail)

	var contributorModel emails.Contributor

	if contributor.LFUsername != "" && contributor.LFEmail != "" && len(projectSFs) > 0 {
		contributorModel.Email = contributor.LFEmail
		contributorModel.Username = contributor.LFUsername
		contributorModel.EmailLabel = utils.EmailLabel
		contributorModel.UsernameLabel = utils.UserLabel
		s.SendEmailToCLAManagerDesignee(ctx, ToCLAManagerDesigneeModel{
			designeeName:     user.Name,
			designeeEmail:    userEmail,
			companyName:      organization.Name,
			projectNames:     projectSFs,
			projectSFIDs:     projectSFIDs,
			contributorModel: contributorModel,
		})
	} else {
		contributorModel = getContributorPublicEmail(contributor)
		s.SendEmailToCLAManagerDesignee(ctx, ToCLAManagerDesigneeModel{
			designeeName:     user.Name,
			designeeEmail:    userEmail,
			companyName:      organization.Name,
			projectNames:     projectSFs,
			projectSFIDs:     projectSFIDs,
			contributorModel: contributorModel,
		})
	}

	log.Debugf("CLA Manager designee created : %+v", designeeScopes)

	return designeeScopes, nil

}

func (s *service) ProjectCompanySignedOrNot(ctx context.Context, signedAtFoundation bool, projectCLAGroups []*projects_cla_groups.ProjectClaGroup, companyModel *v1Models.Company) error {
	f := logrus.Fields{
		"functionName":       "cla_manager.service.ProjectCompanySignedOrNot",
		utils.XREQUESTID:     ctx.Value(utils.XREQUESTID),
		"signedAtFoundation": signedAtFoundation,
		"companyID":          companyModel.CompanyID,
		"companySFID":        companyModel.CompanyExternalID,
		"companyName":        companyModel.CompanyName,
	}

	if signedAtFoundation {
		foundationSFID := projectCLAGroups[0].FoundationSFID

		log.WithFields(f).Debugf("checking if company/project is signed with CLA managers...")
		isSigned, signedErr := s.isSigned(ctx, companyModel, foundationSFID)
		if signedErr != nil {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - %s", signedErr)
			log.WithFields(f).Warn(msg)
			return signedErr
		}
		if isSigned {
			msg := fmt.Sprintf("EasyCLA - 400 Bad Request - Project: %s is already signed", foundationSFID)
			log.WithFields(f).Warn(msg)
			return ErrProjectSigned
		}
	} else {
		for _, pcg := range projectCLAGroups {
			log.WithFields(f).Debugf("checking if company/project is signed with CLA managers...")
			isSigned, signedErr := s.isSigned(ctx, companyModel, pcg.ProjectSFID)
			if signedErr != nil {
				msg := fmt.Sprintf("EasyCLA - 400 Bad Request - %s", signedErr)
				log.WithFields(f).Warn(msg)
				return signedErr
			}
			if isSigned {
				msg := fmt.Sprintf("EasyCLA - 400 Bad Request - Project: %s is already signed", pcg.ProjectSFID)
				log.WithFields(f).Warn(msg)
				return ErrProjectSigned
			}
		}
	}
	return nil
}

func validateInviteCompanyAdmin(contactAdmin bool, userEmail string, name string, contributor *v1User.User) error {
	if contributor.UserID == "" {
		return ErrCLAUserNotFound
	}
	if !contactAdmin {
		if name == "" {
			return errors.New("name is required")
		}
		if userEmail == "" {
			return errors.New("email is required")
		}
	}
	return nil
}

func (s *service) NotifyCLAManagers(ctx context.Context, notifyCLAManagers *models.NotifyClaManagerList, CorporateConsoleV2URL string) error {
	f := logrus.Fields{
		"functionName":      "cla_manager.service.NotifyCLAManagers",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"companyName":       notifyCLAManagers.CompanyName,
		"signingEntityName": notifyCLAManagers.SigningEntityName,
		"userID":            notifyCLAManagers.UserID,
		"claGroupName":      notifyCLAManagers.ClaGroupID,
	}
	// Search for Easy CLA User
	log.WithFields(f).Debugf("Getting user by ID: %s", notifyCLAManagers.UserID)
	userModel, userErr := s.easyCLAUserService.GetUser(notifyCLAManagers.UserID)
	if userErr != nil {
		msg := fmt.Sprintf("Problem getting user by ID: %s ", notifyCLAManagers.UserID)
		log.WithFields(f).Warn(msg)
		return ErrCLAUserNotFound
	}

	// Get mappings
	var projectSFIDs []string
	pcgs, pcgErr := s.projectCGRepo.GetProjectsIdsForClaGroup(ctx, notifyCLAManagers.ClaGroupID)
	if pcgErr != nil {
		log.WithFields(f).Warnf("problem getting cla_group_mappings by claGroupID: %s ", notifyCLAManagers.ClaGroupID)
		return pcgErr
	}

	for _, pcg := range pcgs {
		projectSFIDs = append(projectSFIDs, pcg.ProjectSFID)
	}

	log.Debugf("Sending notification emails to CLA Managers: %+v", notifyCLAManagers.List)
	for _, claManager := range notifyCLAManagers.List {
		s.SendEmailToCLAManager(ctx, &EmailToCLAManagerModel{
			Contributor:         userModel,
			CLAManagerName:      claManager.Name,
			CLAManagerEmail:     claManager.Email.String(),
			CompanyName:         notifyCLAManagers.CompanyName,
			CorporateConsoleURL: CorporateConsoleV2URL,
		}, projectSFIDs)
	}

	return nil
}

// getBestUserName is a helper function to extract what information we can from the user record for purposes of displaying the user's name
func getBestUserName(model *v1Models.User) string {
	if model.Username != "" {
		return model.Username
	}

	if model.GithubUsername != "" {
		return model.GithubUsername
	}

	if model.LfUsername != "" {
		return model.LfUsername
	}

	return "User Name Unknown"
}

func getContributorPublicEmail(model *v1User.User) emails.Contributor {
	var contributorModel emails.Contributor
	if model.LFUsername != "" {
		contributorModel.Username = model.LFUsername
		contributorModel.UsernameLabel = utils.UserLabel
	}

	if model.LFEmail != "" {
		contributorModel.Email = model.LFEmail
		contributorModel.EmailLabel = utils.EmailLabel
	}

	if contributorModel.Username == "" {
		contributorModel.Username = model.UserGithubUsername
		contributorModel.UsernameLabel = utils.GitHubUserLabel
	}

	if contributorModel.Email == "" && len(model.UserEmails) > 0 {
		for _, email := range model.UserEmails {
			if strings.Contains(email, "users.noreply.github.com") {
				continue
			}
			contributorModel.Email = email
			contributorModel.EmailLabel = utils.GitHubEmailLabel
		}
	}
	return contributorModel
}

// getFormattedUserDetails is a helper function to extract what information we can from the user record for purposes of displaying the user's information
func getFormattedUserDetails(model *v1Models.User) string {
	var details []string
	if model.Username != "" {
		details = append(details, fmt.Sprintf("User Name: %s", model.Username))
	}

	if model.GithubUsername != "" {
		details = append(details, fmt.Sprintf("GitHub User Name: %s", model.GithubUsername))
	}

	if model.LfUsername != "" {
		details = append(details, fmt.Sprintf("LF Login: %s", model.LfUsername))
	}

	if model.LfEmail != "" {
		details = append(details, fmt.Sprintf("LF Email: %s", model.LfEmail))
	}

	if model.Emails != nil {
		details = append(details, fmt.Sprintf("Emails: %s", strings.Join(model.Emails, ", ")))
	}

	return strings.Join(details, ", ")
}

// isSigned is a helper function to check if project/claGroup is signed
func (s *service) isSigned(ctx context.Context, companyModel *v1Models.Company, projectID string) (bool, error) {
	f := logrus.Fields{
		"functionName":   "cla_manager.service.isSigned",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      companyModel.CompanyID,
		"companyName":    companyModel.CompanyName,
		"companySFID":    companyModel.CompanyExternalID,
		"projectID":      projectID,
	}

	f["companyID"] = companyModel.CompanyID
	f["companyName"] = companyModel.CompanyName
	log.WithFields(f).Debug("loading CLA Managers for company/project")
	claManagers, err := s.v2CompanyService.GetCompanyProjectCLAManagers(ctx, companyV1toV2(companyModel), projectID)
	if err != nil {
		msg := fmt.Sprintf("EasyCLA - 400 Bad Request : %v", err)
		log.WithFields(f).Warn(msg)
		return false, err
	}

	if len(claManagers.List) > 0 {
		log.WithFields(f).Warnf("CLA Group signed for company/project - %d CLA Managers", len(claManagers.List))
		return true, nil
	}

	return false, nil
}

// buildErrorMessage helper function to build an error message
func buildErrorMessage(errPrefix string, claGroupID string, params cla_manager.CreateCLAManagerParams, err error) string {
	return fmt.Sprintf("%s - problem creating new CLA Manager Request using company ID: %s, project ID: %s, first name: %s, last name: %s, user email: %s, error: %+v",
		errPrefix, params.CompanyID, claGroupID, *params.Body.FirstName, *params.Body.LastName, *params.Body.UserEmail, err)
}

func companyV1toV2(v1CompanyModel *v1Models.Company) *models.Company {
	return &models.Company{
		CompanyACL:        v1CompanyModel.CompanyACL,
		CompanyID:         v1CompanyModel.CompanyID,
		CompanyExternalID: v1CompanyModel.CompanyExternalID,
		CompanyName:       v1CompanyModel.CompanyName,
		SigningEntityName: v1CompanyModel.SigningEntityName,
		CompanyManagerID:  v1CompanyModel.CompanyManagerID,
		Note:              v1CompanyModel.Note,
		Created:           v1CompanyModel.Created,
		Updated:           v1CompanyModel.Updated,
		Version:           v1CompanyModel.Version,
	}
}

// GetNonNoReplyUserEmail tries to fetch an email which doesn't have noreply string in it
// but if it's the only one we have it'll still be returned
func GetNonNoReplyUserEmail(userEmails []string) string {
	if len(userEmails) == 0 {
		return ""
	}

	var excludedEmails []string

	for _, email := range userEmails {
		if strings.HasSuffix(email, excludedNoReplyEmails) {
			excludedEmails = append(excludedEmails, email)
			continue
		}
		return email
	}

	if len(excludedEmails) > 0 {
		return excludedEmails[0]
	}

	return ""
}
