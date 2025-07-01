// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package organization_service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/linuxfoundation/easycla/cla-backend-go/events"
	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"

	runtimeClient "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/token"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/models"
)

// Client is client for organization_service
type Client struct {
	cl *client.OrganizationServiceAPI
}

const (
	projectOrganization = "project|organization"
)

var (
	organizationServiceClient *Client
	v1EventService            events.Service
)

// InitClient initializes the user_service client
func InitClient(APIGwURL string, eventService events.Service) {
	APIGwURL = strings.ReplaceAll(APIGwURL, "https://", "")
	organizationServiceClient = &Client{
		cl: client.NewHTTPClientWithConfig(strfmt.Default, &client.TransportConfig{
			Host:     APIGwURL,
			BasePath: "organization-service",
			Schemes:  []string{"https"},
		}),
	}
	v1EventService = eventService
}

// GetClient return user_service client
func GetClient() *Client {
	return organizationServiceClient
}

// CreateOrgUserRoleOrgScope attached role scope for particular org and user
func (osc *Client) CreateOrgUserRoleOrgScope(ctx context.Context, emailID string, organizationID string, roleID string) error {
	f := logrus.Fields{
		"functionName":   "organization_service.CreateOrgUserRoleOrgScope",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"emailID":        emailID,
		"organizationID": organizationID,
		"roleID":         roleID,
	}

	params := &organizations.CreateOrgUsrRoleScopesParams{
		CreateRoleScopes: &models.CreateRolescopes{
			EmailAddress: emailID,
			ObjectID:     &organizationID,
			ObjectType:   aws.String("organization"),
			RoleID:       &roleID,
		},
		SalesforceID: organizationID,
		Context:      ctx,
	}

	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token")
		return err
	}

	clientAuth := runtimeClient.BearerToken(tok)
	log.WithFields(f).Debug("calling org service CreateOrgUsrRoleScopes")
	result, err := osc.cl.Organizations.CreateOrgUsrRoleScopes(params, clientAuth)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to assign user to organization")
		_, ok := err.(*organizations.CreateOrgUsrRoleScopesConflict)
		if !ok {
			return err
		}
		log.WithFields(f).Warn("the role already assigned for the user skipping")
	}

	log.WithFields(f).Debugf("Successfully assigned user to organization, result: %#v", result)
	return nil
}

// IsCompanyOwner checks if User is company owner
func (osc *Client) IsCompanyOwner(ctx context.Context, userSFID string, orgs []string) (bool, error) {
	f := logrus.Fields{
		"functionName":   "organization_service.IsCompanyOwner",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"userSFID":       userSFID,
		"orgs":           strings.Join(orgs, ","),
	}

	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token")
		return false, err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	for index, org := range orgs {
		params := &organizations.ListOrgUsrAdminScopesParams{
			SalesforceID: org,
			Context:      ctx,
		}
		result, scopeErr := osc.cl.Organizations.ListOrgUsrAdminScopes(params, clientAuth)
		if scopeErr != nil {
			msg := fmt.Sprintf("error : %+v ", scopeErr)
			log.WithFields(f).WithError(scopeErr).Warn(msg)
			//Ensure to check the 2 organizations in question
			if index == 0 {
				continue
			}
			if _, ok := scopeErr.(*organizations.ListOrgUsrAdminScopesNotFound); ok {
				return false, nil
			}
			return false, scopeErr
		}
		data := result.Payload
		for _, userRole := range data.Userroles {
			if userRole.Contact.ID == userSFID {
				for _, roleScopes := range userRole.RoleScopes {
					if roleScopes.RoleName == "company-owner" {
						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}

// IsUserHaveRoleScope checks if user have required role and scope
func (osc *Client) IsUserHaveRoleScope(ctx context.Context, roleName string, userSFID string, organizationID string, projectSFID string) (bool, error) {
	f := logrus.Fields{
		"functionName":   "organization_service.IsUserHaveRoleScope",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"roleName":       roleName,
		"userSFID":       userSFID,
		"organizationID": organizationID,
		"projectSFID":    projectSFID,
	}

	objectID := fmt.Sprintf("%s|%s", projectSFID, organizationID)
	var offset int64
	var pageSize int64 = 1000
	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token")
		return false, err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	for {
		params := &organizations.ListOrgUsrServiceScopesParams{
			Offset:       aws.String(strconv.FormatInt(offset, 10)),
			PageSize:     aws.String(strconv.FormatInt(pageSize, 10)),
			SalesforceID: organizationID,
			Rolename:     []string{roleName},
			Context:      ctx,
		}
		result, err := osc.cl.Organizations.ListOrgUsrServiceScopes(params, clientAuth)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable get organization user service scopes")
			return false, err
		}
		for _, userRole := range result.Payload.Userroles {
			// loop until we find user
			if userRole.Contact.ID != userSFID {
				continue
			}
			for _, rolescope := range userRole.RoleScopes {
				for _, scope := range rolescope.Scopes {
					if scope.ObjectTypeName == projectOrganization && scope.ObjectID == objectID {
						return true, nil
					}
				}
				return false, nil
			}
			return false, nil
		}
		if result.Payload.Metadata.TotalSize < offset+pageSize {
			break
		}
		offset = offset + pageSize
	}
	return false, nil
}

// CreateOrgUserRoleOrgScopeProjectOrg assigns role scope to user
func (osc *Client) CreateOrgUserRoleOrgScopeProjectOrg(ctx context.Context, emailID string, projectID string, organizationID string, roleID string) error {
	f := logrus.Fields{
		"functionName":   "organization_service.CreateOrgUserRoleOrgScopeProjectOrg",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"projectID":      projectID,
		"organizationID": organizationID,
		"roleID":         roleID,
		"emailID":        emailID,
	}

	params := &organizations.CreateOrgUsrRoleScopesParams{
		CreateRoleScopes: &models.CreateRolescopes{
			EmailAddress: emailID,
			ObjectID:     aws.String(fmt.Sprintf("%s|%s", projectID, organizationID)),
			ObjectType:   aws.String("project|organization"),
			RoleID:       &roleID,
		},
		SalesforceID: organizationID,
		Context:      ctx,
	}
	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token")
		return err
	}

	clientAuth := runtimeClient.BearerToken(tok)
	log.Debugf("CreateOrgUserRoleScope: called with args emailID: %s, projectID: %s, organizationID: %s, roleID: %s", emailID, projectID, organizationID, roleID)
	result, err := osc.cl.Organizations.CreateOrgUsrRoleScopes(params, clientAuth)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("CreateOrgUserRoleScope failed")
		return err
	}

	log.Debugf("result: %#v", result)
	return nil
}

// DeleteRolePermissions removes the specified Org/Project user permissions for with the given role
func (osc *Client) DeleteRolePermissions(ctx context.Context, organizationID, projectID, role string, authUser *auth.User) error {
	f := logrus.Fields{
		"functionName":   "organization_service.DeleteRolePermissions",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"organizationID": organizationID,
		"projectID":      projectID,
		"role":           role,
	}

	// First, query the organization for the list of permissions (scopes)
	scopeResponse, err := osc.ListOrgUserScopes(ctx, organizationID, []string{role})
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem listing org user scopes")
		return err
	}

	// For each result...
	for _, userRoleScopes := range scopeResponse.Userroles {
		userName := userRoleScopes.Contact.Username
		userEmail := userRoleScopes.Contact.EmailAddress

		for _, roleScopes := range userRoleScopes.RoleScopes {
			roleID := roleScopes.RoleID
			for _, scope := range roleScopes.Scopes {
				// Encoded as ProjectID|OrganizationID - split them out
				objectList := strings.Split(scope.ObjectID, "|")
				// check objectID having project|organization scope
				if len(objectList) == 2 {
					if scope.ObjectTypeName == projectOrganization && projectID == objectList[0] {
						log.WithFields(f).Debugf("removing user from role: %s with scope: %s for project: %s and organization: %s",
							roleScopes.RoleName, scope.ObjectName, projectID, organizationID)
						delErr := osc.DeleteOrgUserRoleOrgScopeProjectOrg(ctx, organizationID, roleID, scope.ScopeID, &userName, &userEmail)
						if delErr != nil {
							f["userName"] = userName
							f["userEmail"] = userEmail
							log.WithFields(f).Warnf("problem removing user from role, error: %+v", err)
							return delErr
						}

						// Log Event...
						v1EventService.LogEvent(&events.LogEventArgs{
							EventType:   events.ClaManagerRoleDeleted,
							ProjectID:   projectID,
							ProjectSFID: projectID,
							CompanyID:   organizationID,
							LfUsername:  authUser.UserName,
							UserID:      authUser.UserName,
							EventData: &events.ClaManagerRoleDeletedData{
								Role:      role,                 // cla-manager
								Scope:     scope.ObjectTypeName, // project|organization
								UserName:  userName,             // bstonedev
								UserEmail: userEmail,            // bstone+dev@linuxfoundation.org
							},
						})
					}
				}
			}
		}
	}
	return nil
}

// DeleteOrgUserRoleOrgScopeProjectOrg removes role scope for user
func (osc *Client) DeleteOrgUserRoleOrgScopeProjectOrg(ctx context.Context, organizationID string, roleID string, scopeID string, userName *string, userEmail *string) error {
	f := logrus.Fields{
		"functionName":   "organization_service.DeleteOrgUserRoleOrgScopeProjectOrg",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"organizationID": organizationID,
		"roleID":         roleID,
		"scopeID":        scopeID,
		"userName":       *userName,
		"userEmail":      *userEmail,
	}

	params := &organizations.DeleteOrgUsrRoleScopesParams{
		SalesforceID: organizationID,
		RoleID:       roleID,
		ScopeID:      scopeID,
		XUSERNAME:    userName,
		XEMAIL:       userEmail,
		Context:      ctx,
	}
	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token")
		return err
	}

	clientAuth := runtimeClient.BearerToken(tok)
	log.WithFields(f).Debugf("removing organization user roles with organizationID: %s, roleID: %s, scopeID: %s",
		organizationID, roleID, scopeID)
	result, deleteErr := osc.cl.Organizations.DeleteOrgUsrRoleScopes(params, clientAuth)
	if deleteErr != nil {
		log.WithFields(f).Warnf("DeleteOrgUserRoleOrgScopeProjectOrg failed, error: %+v", deleteErr)
		return deleteErr
	}

	log.WithFields(f).Debugf("result: %#v", result)
	return nil
}

// GetScopeID will return scopeID for a give role
func (osc *Client) GetScopeID(ctx context.Context, organizationID string, projectID string, roleName string, objectTypeName string, userLFID string) (string, error) {
	f := logrus.Fields{
		"functionName":   "organization_service.GetScopeID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"organizationID": organizationID,
		"projectID":      projectID,
		"roleName":       roleName,
		"objectTypeName": objectTypeName,
		"userLFID":       userLFID,
	}
	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token")
		return "", err
	}
	params := &organizations.ListOrgUsrServiceScopesParams{
		SalesforceID: organizationID,
		Context:      ctx,
	}
	clientAuth := runtimeClient.BearerToken(tok)
	result, err := osc.cl.Organizations.ListOrgUsrServiceScopes(params, clientAuth)
	if err != nil {
		return "", err
	}
	data := result.Payload
	for _, userRole := range data.Userroles {
		// Check scopes for given user
		if userRole.Contact.Username == userLFID {
			for _, roleScopes := range userRole.RoleScopes {
				if roleScopes.RoleName == roleName {
					for _, scope := range roleScopes.Scopes {
						// Check object ID and and objectTypeName
						objectList := strings.Split(scope.ObjectID, "|")
						// check objectID having project|organization scope
						if len(objectList) == 2 {
							if scope.ObjectTypeName == objectTypeName && projectID == objectList[0] {
								return scope.ScopeID, nil
							}
						}
					}
				}
			}
		}
	}
	return "", nil
}

// SearchOrganization search organization by name. It will return
// array of organization matching with the orgName.
func (osc *Client) SearchOrganization(ctx context.Context, orgName string, websiteName string, filter string) ([]*models.Organization, error) {
	f := logrus.Fields{
		"functionName":   "organization_service.SearchOrganization",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"orgName":        orgName,
		"websiteName":    websiteName,
		"filter":         filter,
	}
	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token")
		return nil, err
	}
	var offset int64
	var pageSize int64 = 1000
	clientAuth := runtimeClient.BearerToken(tok)
	var orgs []*models.Organization
	for {
		params := &organizations.SearchOrgParams{
			Name:         aws.String(orgName),
			Website:      aws.StringValueSlice([]*string{&websiteName}),
			DollarFilter: aws.String(filter),
			Offset:       aws.String(strconv.FormatInt(offset, 10)),
			PageSize:     aws.String(strconv.FormatInt(pageSize, 10)),
			Context:      context.TODO(),
		}
		result, err := osc.cl.Organizations.SearchOrg(params, clientAuth)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("unable to search organization with params: %+v", params)
			return nil, err
		}
		orgs = append(orgs, result.Payload.Data...)
		if result.Payload.Metadata.TotalSize > offset+pageSize {
			offset += pageSize
		} else {
			break
		}
	}
	return orgs, nil
}

// GetOrganization gets organization from organization id
func (osc *Client) GetOrganization(ctx context.Context, orgID string) (*models.Organization, error) {
	f := logrus.Fields{
		"functionName":   "organization_service.GetOrganization",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"orgID":          orgID,
	}
	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token")
		return nil, err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	params := &organizations.GetOrgParams{
		SalesforceID: orgID,
		Context:      ctx,
	}
	result, err := osc.cl.Organizations.GetOrg(params, clientAuth)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to get organization with params: %+v", params)
		return nil, err
	}
	return result.Payload, nil
}

// ListOrgUserAdminScopes returns admin role scope of organization
func (osc *Client) ListOrgUserAdminScopes(ctx context.Context, orgID string, role *string) (*models.UserrolescopesList, error) {
	f := logrus.Fields{
		"functionName":   "organization_service.ListOrgUserAdminScopes",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"orgID":          orgID,
		"role":           utils.StringValue(role),
	}
	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token")
		return nil, err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	params := &organizations.ListOrgUsrAdminScopesParams{
		SalesforceID: orgID,
		Context:      ctx,
	}
	if role != nil {
		params.Rolename = []string{*role}
	}
	result, err := osc.cl.Organizations.ListOrgUsrAdminScopes(params, clientAuth)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to list organization user admin scopes with params: %+v", params)
		return nil, err
	}
	return result.Payload, nil
}

// ListOrgUserScopes returns role scope of organization, rolename is optional filter
func (osc *Client) ListOrgUserScopes(ctx context.Context, orgID string, roleName []string) (*models.UserrolescopesList, error) {
	f := logrus.Fields{
		"functionName":   "organization_service.ListOrgUserScopes",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"orgID":          orgID,
		"roleName":       strings.Join(roleName, ","),
	}
	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token")
		return nil, err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	params := &organizations.ListOrgUsrServiceScopesParams{
		SalesforceID: orgID,
		Context:      ctx,
	}
	if len(roleName) != 0 {
		params.Rolename = roleName
	}

	result, err := osc.cl.Organizations.ListOrgUsrServiceScopes(params, clientAuth)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to list organization user service scopes with params: %+v", params)
		return nil, err
	}

	return result.Payload, nil
}

// CreateOrg creates company based on name and website with additional data for required fields
func (osc *Client) CreateOrg(ctx context.Context, companyName, signingEntityName, companyWebsite string) (*models.Organization, error) {
	f := logrus.Fields{
		"functionName":      "organization_service.CreateOrg",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"companyName":       companyName,
		"signingEntityName": signingEntityName,
		"companyWebsite":    companyWebsite,
	}

	var org *models.Organization

	tok, tokenErr := token.GetToken()
	if tokenErr != nil {
		log.WithFields(f).WithError(tokenErr).Warn("unable to fetch token")
		return nil, tokenErr
	}

	// If not specified, use the company name as the signing entity name
	if signingEntityName == "" {
		signingEntityName = companyName
	}

	//Lookup Org based on domain
	lookupOrg, lookupErr := osc.SearchOrgLookup(ctx, nil, &companyWebsite)
	if lookupErr != nil {
		log.WithFields(f).WithError(lookupErr).Warn("unable to search for existing company using company website value")
		if _, ok := lookupErr.(*organizations.LookupNotFound); !ok {
			return nil, lookupErr
		}
	}

	if lookupOrg != nil && lookupOrg.Payload.ID != "" {
		// Get org based on ID
		var updateErr error
		existingOrg, existingOrgErr := osc.GetOrganization(ctx, lookupOrg.Payload.ID)
		if existingOrgErr != nil {
			log.WithFields(f).WithError(existingOrgErr).Warnf("unable to get organization : %s ", lookupOrg.Payload.ID)
			return nil, existingOrgErr
		}
		org, updateErr = osc.UpdateOrg(ctx, existingOrg, signingEntityName)
		if updateErr != nil {
			log.WithFields(f).WithError(updateErr).Warn("unable to update for existing company")
			return nil, updateErr
		}

	} else {
		// use linux foundation logo as default
		linuxFoundation, err := osc.SearchOrganization(ctx, utils.TheLinuxFoundation, "", "")
		if err != nil || len(linuxFoundation) == 0 {
			log.WithFields(f).WithError(err).Warn("unable to search Linux Foundation organization")
			return nil, err
		}

		clientAuth := runtimeClient.BearerToken(tok)
		logoURL := linuxFoundation[0].LogoURL
		f["logoURL"] = logoURL

		params := &organizations.CreateOrgParams{
			Org: &models.CreateOrg{
				Name:              &companyName,
				Website:           &companyWebsite,
				LogoURL:           logoURL,
				SigningEntityName: []string{signingEntityName},
			},
			Context: ctx,
		}

		log.WithFields(f).Debugf("Creating organization with params: %+v", models.CreateOrg{
			Name:              &companyName,
			Website:           &companyWebsite,
			LogoURL:           logoURL,
			SigningEntityName: []string{signingEntityName},
		})
		result, err := osc.cl.Organizations.CreateOrg(params, clientAuth)
		if err != nil {
			log.WithFields(f).WithError(err).Warnf("Failed to create salesforce Company: %s , err: %+v ", companyName, err)
			return nil, err
		}
		log.WithFields(f).Infof("Company: %s  successfuly created ", companyName)

		org = result.Payload
	}
	return org, nil
}

// UpdateOrg updates the company record based on the provided name, signingEntityName, and website
func (osc *Client) UpdateOrg(ctx context.Context, existingCompanyModel *models.Organization, signingEntityName string) (*models.Organization, error) {
	f := logrus.Fields{
		"functionName":      "organization_service.UpdateOrg",
		utils.XREQUESTID:    ctx.Value(utils.XREQUESTID),
		"companyName":       existingCompanyModel.Name,
		"signingEntityName": signingEntityName,
		"companyWebsite":    existingCompanyModel.Link,
	}

	tok, tokenErr := token.GetToken()
	if tokenErr != nil {
		log.WithFields(f).WithError(tokenErr).Warn("unable to fetch token")
		return nil, tokenErr
	}

	signingEntityNames := existingCompanyModel.SigningEntityName
	signingEntityNames = append(signingEntityNames, strings.TrimSpace(signingEntityName))
	// Ensure no duplicates
	signingEntityNames = utils.RemoveDuplicates(signingEntityNames)
	// Sort nicely
	sort.Strings(signingEntityNames)

	clientAuth := runtimeClient.BearerToken(tok)
	params := &organizations.UpdateOrgParams{
		UpdateOrganization: &models.UpdateOrg{
			SigningEntityName: signingEntityNames,
		},
		SalesforceID: existingCompanyModel.ID,
		Context:      ctx,
	}

	log.WithFields(f).Debugf("Update organization with params: %+v", params)
	result, err := osc.cl.Organizations.UpdateOrg(params, clientAuth)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Failed to update salesforce Company: %s, err: %+v",
			existingCompanyModel.Name, err)
		return nil, err
	}
	log.WithFields(f).Infof("Company: %s successfuly updated ", existingCompanyModel.Name)

	return result.Payload, err
}

// ListOrg returns organization
func (osc *Client) ListOrg(ctx context.Context, orgName string) (*models.OrganizationList, error) {
	f := logrus.Fields{
		"functionName":   "organization_service.ListOrg",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"orgName":        orgName,
	}

	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token")
		return nil, err
	}
	clientAuth := runtimeClient.BearerToken(tok)
	params := &organizations.ListOrgParams{
		Name:    &orgName,
		Context: ctx,
	}

	result, err := osc.cl.Organizations.ListOrg(params, clientAuth)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Failed to list organizations using params: %+v", params)
		return nil, err
	}
	return result.Payload, nil
}

// SearchOrgLookup returns organization
func (osc *Client) SearchOrgLookup(ctx context.Context, orgName, websiteName *string) (*organizations.LookupOK, error) {
	f := logrus.Fields{
		"functionName": "organization_service.Lookup",
		"orgName":      utils.StringValue(orgName),
		"websiteName":  utils.StringValue(websiteName),
	}

	tok, err := token.GetToken()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to fetch token")
		return nil, err
	}

	clientAuth := runtimeClient.BearerToken(tok)
	params := &organizations.LookupParams{
		Context: ctx,
	}
	if orgName != nil {
		params.Name = orgName
	}
	if websiteName != nil {
		params.Domain = websiteName
	}
	result, err := osc.cl.Organizations.Lookup(params, clientAuth)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to search organization with params: %+v", params)
		return nil, err
	}

	return result, nil

}
