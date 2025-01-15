// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/communitybridge/easycla/cla-backend-go/utils"
	acs_service "github.com/communitybridge/easycla/cla-backend-go/v2/acs-service"
	organization_service "github.com/communitybridge/easycla/cla-backend-go/v2/organization-service"
	user_service "github.com/communitybridge/easycla/cla-backend-go/v2/user-service"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/aws/aws-lambda-go/events"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

// constants
const (
	CLASignatureType  = "cla"
	CCLASignatureType = "ccla"

	ICLASignatureType = "icla"
	ECLASignatureType = "ecla"
	AddCLAManager     = "add"
	DeleteCLAManager  = "delete"
)

// ErrNoExternalID when company does not have externalID
var ErrNoExternalID = errors.New("company External ID does not exist")

// Signature database model
type Signature struct {
	SignatureID                   string   `json:"signature_id"`
	DateCreated                   string   `json:"date_created"`
	DateModified                  string   `json:"date_modified"`
	SignatureApproved             bool     `json:"signature_approved"`
	SignatureSigned               bool     `json:"signature_signed"`
	SignatureEmbargoAcked         bool     `json:"signature_embargo_acked"`
	SignatureDocumentMajorVersion string   `json:"signature_document_major_version"`
	SignatureDocumentMinorVersion string   `json:"signature_document_minor_version"`
	SignatureReferenceID          string   `json:"signature_reference_id"`
	SignatureReferenceName        string   `json:"signature_reference_name"`
	SignatureReferenceNameLower   string   `json:"signature_reference_name_lower"`
	SignatureProjectID            string   `json:"signature_project_id"`
	SignatureReferenceType        string   `json:"signature_reference_type"`
	SignatureType                 string   `json:"signature_type"`
	SignatureUserCompanyID        string   `json:"signature_user_ccla_company_id"`
	EmailWhitelist                []string `json:"email_whitelist"`
	DomainWhitelist               []string `json:"domain_whitelist"`
	GitHubWhitelist               []string `json:"github_whitelist"`
	GitHubOrgWhitelist            []string `json:"github_org_whitelist"`
	SignatureACL                  []string `json:"signature_acl"`
	SigtypeSignedApprovedID       string   `json:"sigtype_signed_approved_id"`
	UserGithubUsername            string   `json:"user_github_username"`
	UserLFUsername                string   `json:"user_lf_username"`
	UserName                      string   `json:"user_name"`
	UserEmail                     string   `json:"user_email"`
	SignedOn                      string   `json:"signed_on"`
}

// Assign Contributor role upon CCLA or CCLA/ICLA signing
func (s *service) SignatureAssignContributorEvent(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "SignatureAssignContributorEvent",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	log.WithFields(f).Debug("processing signature event to assign contributor...")

	// Decode the pre-update and post-update signature record details
	var newSignature, oldSignature Signature
	err := unmarshalStreamImage(event.Change.OldImage, &oldSignature)
	if err != nil {
		log.WithFields(f).Warnf("problem decoding pre-update signature, error: %+v", err)
		return err
	}
	err = unmarshalStreamImage(event.Change.NewImage, &newSignature)
	if err != nil {
		log.WithFields(f).Warnf("problem decoding post-update signature, error: %+v", err)
		return err
	}

	// Add some details for our logger
	f["id"] = newSignature.SignatureID
	f["type"] = newSignature.SignatureType
	f["referenceID"] = newSignature.SignatureReferenceID
	f["referenceName"] = newSignature.SignatureReferenceName
	f["referenceType"] = newSignature.SignatureReferenceType
	f["projectID"] = newSignature.SignatureProjectID
	f["approved"] = newSignature.SignatureApproved
	f["signed"] = newSignature.SignatureSigned
	f["embargo_acked"] = newSignature.SignatureEmbargoAcked

	if !oldSignature.SignatureSigned && newSignature.SignatureSigned {
		log.WithFields(f).Debug("signature is now signed - assigning contributor...")
		err := s.assignContributor(ctx, newSignature, f)
		if err != nil {
			return err
		}
	}

	return nil
}

// should be called when we modify signature
func (s *service) SignatureSignedEvent(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "SignatureSignedEvent",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	log.WithFields(f).Debug("Processing signature signed event to modify the ACL...")

	// Decode the pre-update and post-update signature record details
	var newSignature, oldSignature Signature
	err := unmarshalStreamImage(event.Change.OldImage, &oldSignature)
	if err != nil {
		log.WithFields(f).Warnf("problem decoding pre-update signature, error: %+v", err)
		return err
	}
	err = unmarshalStreamImage(event.Change.NewImage, &newSignature)
	if err != nil {
		log.WithFields(f).Warnf("problem decoding post-update signature, error: %+v", err)
		return err
	}

	// Add some details for our logger
	f["id"] = newSignature.SignatureID
	f["type"] = newSignature.SignatureType
	f["referenceID"] = newSignature.SignatureReferenceID
	f["referenceName"] = newSignature.SignatureReferenceName
	f["referenceType"] = newSignature.SignatureReferenceType
	f["projectID"] = newSignature.SignatureProjectID
	f["approved"] = newSignature.SignatureApproved
	f["signed"] = newSignature.SignatureSigned
	f["embargo_acked"] = newSignature.SignatureEmbargoAcked

	// check if signature signed event is received
	if !oldSignature.SignatureSigned && newSignature.SignatureSigned {
		log.WithFields(f).Debugf("processing signature signed event for signature type: %s...", newSignature.SignatureType)

		// Update the signed on date
		log.WithFields(f).Debug("updating signed on date...")
		err = s.signatureRepo.AddSignedOn(ctx, newSignature.SignatureID)
		if err != nil {
			log.WithFields(f).Warnf("failed to add signed_on date/time to signature, error: %+v", err)
		}

		// If oldSigACL CCLA signature...
		if newSignature.SignatureType == CCLASignatureType {
			log.WithFields(f).Debugf("processing signature type: %s with %d CLA Managers...",
				newSignature.SignatureType, len(newSignature.SignatureACL))

			if len(newSignature.SignatureACL) == 0 {
				log.WithFields(f).Warn("initial cla manager details not found")
				return errors.New("initial cla manager details not found")
			}

			log.WithFields(f).Debugf("loading company from signature by companyID: %s...", newSignature.SignatureReferenceID)
			companyModel, err := s.companyRepo.GetCompany(ctx, newSignature.SignatureReferenceID)
			if err != nil {
				log.WithFields(f).Warnf("failed to lookup company from signature by companyID: %s, error: %+v",
					newSignature.SignatureReferenceID, err)
				return err
			}
			if companyModel == nil {
				msg := fmt.Sprintf("failed to lookup company from signature by companyID: %s, not found",
					newSignature.SignatureReferenceID)
				log.WithFields(f).Warn(msg)
				return errors.New(msg)
			}
			log.WithFields(f).Debugf("loaded company '%s' from signature by companyID: %s with companySFID: %s...",
				companyModel.CompanyName, companyModel.CompanyID, companyModel.CompanyExternalID)

			// We should have the company SFID...
			if companyModel.CompanyExternalID == "" {
				msg := fmt.Sprintf("company %s (%s) does not have oldSigACL SF Organization ID - unable to update permissions",
					companyModel.CompanyName, companyModel.CompanyID)
				log.WithFields(f).Warn(msg)
				return errors.New(msg)
			}

			// Load the list of SF projects associated with this CLA Group
			log.WithFields(f).Debugf("querying SF projects for CLA Group: %s", newSignature.SignatureProjectID)
			projectCLAGroups, err := s.projectsClaGroupRepo.GetProjectsIdsForClaGroup(ctx, newSignature.SignatureProjectID)
			log.WithFields(f).Debugf("found %d SF projects for CLA Group: %s", len(projectCLAGroups), newSignature.SignatureProjectID)

			// Only proceed if we have one or more SF projects - otherwise, we can't assign and cleanup/adjust roles
			if len(projectCLAGroups) == 0 {
				log.WithFields(f).Warnf("unable to assign initial %s role or cleanup existing %s roles - no SF projects assigned to CLA group",
					utils.CLAManagerRole, utils.CLADesigneeRole)
				return nil
			}

			// We have oldSigACL separate routine to help assign the CLA Manager Role - it's oldSigACL bit wasteful as it
			// loads the signature and other details again.
			// Kick off oldSigACL go routine to set the cla manager role
			// Set the CLA manager permissions
			log.WithFields(f).Debug("assigning the initial CLA manager")
			err = s.SetInitialCLAManagerACSPermissions(ctx, newSignature.SignatureID)
			if err != nil {
				log.WithFields(f).Warnf("failed to set initial cla manager, error: %+v", err)
				return err
			}

			var eg errgroup.Group

			// Remove designee role based on project type(foundation or Project level)
			foundationSFID := projectCLAGroups[0].FoundationSFID
			signedAtFoundation, signedErr := s.projectService.SignedAtFoundationLevel(ctx, foundationSFID)
			if signedErr != nil {
				log.WithFields(f).Debugf("failed to check signed level for ID: %s ", foundationSFID)
				return signedErr
			}
			if signedAtFoundation {
				log.WithFields(f).Debugf("removing existing %s role for project: '%s' (%s) and company: '%s' (%s)",
					utils.CLADesigneeRole, projectCLAGroups[0].ProjectName, foundationSFID, companyModel.CompanyName, companyModel.CompanyExternalID)
				err = s.removeCLAPermissionsByProjectOrganizationRole(ctx, foundationSFID, companyModel.CompanyExternalID, []string{utils.CLADesigneeRole})
				if err != nil {
					log.WithFields(f).Warnf("failed to remove %s roles for project: '%s' (%s) and company: '%s' (%s), error: %+v",
						utils.CLADesigneeRole, projectCLAGroups[0].ProjectName, foundationSFID, companyModel.CompanyName, companyModel.CompanyExternalID, err)
					return err
				}
			} else {
				for _, projectCLAGroup := range projectCLAGroups {
					pcg := projectCLAGroup // make a copy of the loop variable to use in the closure, avoids the loopclosure: loop variable projectCLAGroup captured by func literal lint error
					eg.Go(func() error {
						// Remove any roles that were previously assigned for cla-manager-designee
						log.WithFields(f).Debugf("removing existing %s role for project: '%s' (%s) and company: '%s' (%s)",
							utils.CLADesigneeRole, pcg.ProjectName, pcg.ProjectSFID, companyModel.CompanyName, companyModel.CompanyExternalID)
						err = s.removeCLAPermissionsByProjectOrganizationRole(ctx, pcg.ProjectSFID, companyModel.CompanyExternalID, []string{utils.CLADesigneeRole})
						if err != nil {
							log.WithFields(f).Warnf("failed to remove %s roles for project: '%s' (%s) and company: '%s' (%s), error: %+v",
								utils.CLADesigneeRole, pcg.ProjectName, pcg.ProjectSFID, companyModel.CompanyName, companyModel.CompanyExternalID, err)
							return err
						}

						return nil
					})
				}
				// Wait for the go routines to finish
				log.WithFields(f).Debug("waiting for role assignment and cleanup...")
				var lastRoleErr error
				if roleErr := eg.Wait(); roleErr != nil {
					log.WithFields(f).Warnf("encountered error while processing roles: %+v", roleErr)
					lastRoleErr = roleErr
				}
				// Could be nil or the last error encountered
				return lastRoleErr
			}
		}
	}

	return nil
}

// SignatureAddSigTypeSignedApprovedID function should be called when new icla, ecla signature added
func (s *service) SignatureAddSigTypeSignedApprovedID(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "SignatureAddSigTypeSignedApprovedID",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	log.WithFields(f).Debug("processing signature event - adding signature type signed approved id...")
	var newSig Signature
	var sigType string
	var id string
	err := unmarshalStreamImage(event.Change.NewImage, &newSig)
	if err != nil {
		return err
	}
	switch {
	case newSig.SignatureType == CCLASignatureType:
		sigType = CCLASignatureType
		id = newSig.SignatureReferenceID
	case newSig.SignatureType == CLASignatureType && newSig.SignatureUserCompanyID == "":
		sigType = ICLASignatureType
		id = newSig.SignatureReferenceID
	case newSig.SignatureType == CLASignatureType && newSig.SignatureUserCompanyID != "":
		sigType = ECLASignatureType
		id = newSig.SignatureUserCompanyID
		log.WithFields(f).Debugf("assigning contributor role for signature: %s ", newSig.SignatureID)
		err = s.assignContributor(ctx, newSig, f)
		if err != nil {
			return err
		}
	default:
		log.WithFields(f).Warnf("setting sigtype_signed_approved_id for signature: %s failed", newSig.SignatureID)
		return errors.New("invalid signature in SignatureAddSigTypeSignedApprovedID")
	}
	val := fmt.Sprintf("%s#%v#%v#%s", sigType, newSig.SignatureSigned, newSig.SignatureApproved, id)
	if newSig.SigtypeSignedApprovedID == val {
		return nil
	}
	log.WithFields(f).Debugf("setting sigtype_signed_approved_id for signature: %s", newSig.SignatureID)
	err = s.signatureRepo.AddSigTypeSignedApprovedID(ctx, newSig.SignatureID, val)
	if err != nil {
		return err
	}
	return nil
}

func (s *service) SignatureAddUsersDetails(event events.DynamoDBEventRecord) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "SignatureAddUsersDetails",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	var newSig Signature
	err := unmarshalStreamImage(event.Change.NewImage, &newSig)
	if err != nil {
		return err
	}
	if newSig.SignatureReferenceType == "user" && newSig.UserLFUsername == "" && newSig.UserGithubUsername == "" {
		log.WithFields(f).Debugf("adding users details in signature: %s", newSig.SignatureID)
		err = s.signatureRepo.AddUsersDetails(ctx, newSig.SignatureID, newSig.SignatureReferenceID)
		if err != nil {
			log.WithFields(f).Debugf("adding users details in signature: %s failed. error = %s", newSig.SignatureID, err.Error())
		}
	}
	return nil
}

// signature function should be invoked when signature ACL is updated
func (s *service) UpdateCLAPermissions(event events.DynamoDBEventRecord) error {
	f := logrus.Fields{
		"functionName": "v2.dynamo_events.UpdateCLAPermissions",
	}

	// Decode the pre-update and post-update signature record details
	var newSignature, oldSignature Signature
	err := unmarshalStreamImage(event.Change.OldImage, &oldSignature)
	if err != nil {
		log.WithFields(f).Warnf("problem decoding pre-update signature, error: %+v", err)
		return err
	}
	err = unmarshalStreamImage(event.Change.NewImage, &newSignature)
	if err != nil {
		log.WithFields(f).Warnf("problem decoding post-update signature, error: %+v", err)
		return err
	}

	f["id"] = newSignature.SignatureID
	f["type"] = newSignature.SignatureType
	f["referenceID"] = newSignature.SignatureReferenceID
	f["referenceName"] = newSignature.SignatureReferenceName
	f["referenceType"] = newSignature.SignatureReferenceType
	f["projectID"] = newSignature.SignatureProjectID
	f["acl"] = strings.Join(newSignature.SignatureACL, ",")

	log.WithFields(f).Debug("processing signature ACL to identify added/removed managers...")
	managers := utils.SliceDifference(oldSignature.SignatureACL, newSignature.SignatureACL)
	if len(managers) > 0 {
		log.WithFields(f).Debugf("managers to be added/deleted : %+v", managers)
		for _, entry := range managers {
			log.WithFields(f).Debugf("processing difference: %s", entry)
			if utils.StringInSlice(entry, newSignature.SignatureACL) {
				// Assign CLA Manager role
				log.WithFields(f).Debugf("Assigning user %s to the %s role", entry, utils.CLAManagerRole)
				updateErr := s.updateCLAManagerPermissions(newSignature, managers, AddCLAManager)
				if updateErr != nil {
					log.WithFields(f).WithError(updateErr).Warn("problem assigning CLA Manager role")
					return updateErr
				}
			} else {
				// Remove CLA Manager role
				log.WithFields(f).Debugf("Unassigning user %s from the %s role", entry, utils.CLAManagerRole)
				updateErr := s.updateCLAManagerPermissions(newSignature, managers, DeleteCLAManager)
				if updateErr != nil {
					log.WithFields(f).WithError(updateErr).Warn("problem removing CLA Manager role")
					return updateErr
				}
			}
		}
	} else {
		log.WithFields(f).Debugf("No changes in ACL : %+v versus %+v", oldSignature.SignatureACL, newSignature.SignatureACL)
	}
	return nil
}

func (s *service) assignContributor(ctx context.Context, newSignature Signature, f logrus.Fields) error {
	var companyID string
	// Assign company ID based on signature type (CCLA, CCLA|ICLA)
	if newSignature.SignatureType == utils.SignatureTypeCLA && newSignature.SignatureUserCompanyID != "" {
		companyID = newSignature.SignatureUserCompanyID
	} else if newSignature.SignatureType == utils.SignatureTypeCCLA && newSignature.SignatureReferenceID != "" {
		companyID = newSignature.SignatureReferenceID
	}
	if (newSignature.SignatureType == utils.SignatureTypeCLA) || (newSignature.SignatureType == utils.SignatureTypeCCLA) {
		log.WithFields(f).Debugf("processing signature type: %s with %d CLA Managers...",
			newSignature.SignatureType, len(newSignature.SignatureACL))
		companyModel, err := s.companyRepo.GetCompany(ctx, companyID)
		if err != nil {
			log.WithFields(f).Warnf("failed to lookup company from signature by companyID: %s, error: %+v",
				companyID, err)
			return err
		}
		if companyModel == nil {
			msg := fmt.Sprintf("failed to lookup company from signature by companyID: %s, not found",
				companyID)
			log.WithFields(f).Warn(msg)
			return errors.New(msg)
		}
		log.WithFields(f).Debugf("loaded company '%s' from signature by companyID: %s with companySFID: %s...",
			companyModel.CompanyName, companyModel.CompanyID, companyModel.CompanyExternalID)
		log.WithFields(f).Debugf("loaded company '%s' from signature by companyID: %s with companySFID: %s...",
			companyModel.CompanyName, companyModel.CompanyID, companyModel.CompanyExternalID)

		// We should have the company SFID...
		if companyModel.CompanyExternalID == "" {
			msg := fmt.Sprintf("company %s (%s) does not have oldSigACL SF Organization ID - unable to update permissions",
				companyModel.CompanyName, companyModel.CompanyID)
			log.WithFields(f).Warn(msg)
			return errors.New(msg)
		}
		// Load the list of SF projects associated with this CLA Group
		log.WithFields(f).Debugf("querying SF projects for CLA Group: %s", newSignature.SignatureProjectID)
		projectCLAGroups, err := s.projectsClaGroupRepo.GetProjectsIdsForClaGroup(ctx, newSignature.SignatureProjectID)
		log.WithFields(f).Debugf("found %d SF projects for CLA Group: %s", len(projectCLAGroups), newSignature.SignatureProjectID)

		if err != nil {
			log.WithFields(f).Errorf("Unable to query projectCLA groups by claGroupID : %s, error: %+v ", newSignature.SignatureProjectID, err)
			return err
		}

		// Only proceed if we have one or more SF projects - otherwise, we can't assign and cleanup/adjust roles
		if len(projectCLAGroups) == 0 {
			log.WithFields(f).Warnf("no SF projects assigned to CLA group: %s ",
				newSignature.SignatureProjectID)
			return nil
		}
		_, _, err = s.companyService.AssociateContributorByGroup(ctx, companyModel.CompanyExternalID, newSignature.UserEmail, projectCLAGroups, newSignature.SignatureProjectID)

		if err != nil {
			log.WithFields(f).Errorf("unable to create contributor association for user: %s and company: %s ", newSignature.UserEmail, companyModel.CompanyExternalID)
		}
		return nil
	}
	return nil
}

// helper function that assigns acl permissions(v2 specific)
func (s *service) updateCLAManagerPermissions(signature Signature, managers []string, action string) error {
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":  "updateCLAManagerPermissions",
		"signatureID":   signature.SignatureID,
		"AddedManagers": managers,
	}

	userClient := user_service.GetClient()
	orgClient := organization_service.GetClient()

	var companyID string
	// Get Salesforce company
	log.WithFields(f).Debug("Getting Salesforce Company ...")
	if signature.SignatureType == utils.ClaTypeCCLA {
		companyID = signature.SignatureReferenceID
	} else if signature.SignatureType == utils.ClaTypeCCLA && signature.SignatureUserCompanyID != "" {
		companyID = signature.SignatureUserCompanyID
	}
	companyModel, compErr := s.companyService.GetCompanyByID(ctx, companyID)

	if compErr != nil {
		log.WithFields(f).WithError(compErr).Warnf("unable to fetch company %s", companyID)
		return compErr
	}

	if companyModel.CompanyExternalID == "" {
		log.WithFields(f).WithError(ErrNoExternalID).Warnf("ExternalCompany ID does not exist for company: %s", companyID)
		return ErrNoExternalID
	}

	org, orgErr := orgClient.GetOrganization(ctx, companyModel.CompanyExternalID)
	if orgErr != nil {
		log.WithFields(f).WithError(orgErr).Warnf("failed to get organisation for ID: %s ", companyModel.CompanyExternalID)
		return orgErr
	}

	projectCLAGroups, pcgErr := s.projectsClaGroupRepo.GetProjectsIdsForClaGroup(ctx, signature.SignatureProjectID)
	if pcgErr != nil {
		log.WithFields(f).WithError(pcgErr).Warnf("unable to get project mappings for claGroupID: %s ", signature.SignatureProjectID)
		return pcgErr
	}

	// Get Role ID
	acsClient := acs_service.GetClient()

	roleID, roleErr := acsClient.GetRoleID(utils.CLAManagerRole)
	if roleErr != nil {
		log.WithFields(f).WithError(roleErr).Warnf("failed to get roleID for : %s ", utils.CLAManagerRole)
		return roleErr
	}

	log.WithFields(f).Debugf("Role ID for cla-manager-role : %s", roleID)

	foundationSFID := projectCLAGroups[0].FoundationSFID
	signedAtFoundation, signedErr := s.projectService.SignedAtFoundationLevel(ctx, foundationSFID)

	// Create ProjectListing that handles CLA Manager permissions
	projectSFIDList := utils.NewStringSet()
	for _, p := range projectCLAGroups {
		projectSFIDList.Add(p.ProjectSFID)
	}

	if signedErr != nil {
		log.WithFields(f).WithError(signedErr).Warnf("failed to check signed status for foundationID: %s ", foundationSFID)
		return signedErr
	}
	for i := range managers {
		// Get User
		mgr := managers[i]
		lfUser, userErr := userClient.GetUserByUsername(mgr)
		if userErr != nil {
			log.WithFields(f).WithError(userErr).Warnf("Failed to get salesforce user for user: %s", mgr)
			continue
		}
		// Get User email
		email := lfUser.Emails[0].EmailAddress
		if signedAtFoundation {
			scopeID, scopeErr := orgClient.GetScopeID(ctx, org.ID, foundationSFID, utils.CLAManagerRole, utils.ProjectOrgScope, mgr)
			if scopeErr != nil {
				log.WithFields(f).WithError(scopeErr).Warnf("failed to get scopeID for foundation: %s , organization: %s, manager: %s and role: %s ", foundationSFID, org.Name, mgr, utils.CLAManagerRole)
				continue
			}
			if action == AddCLAManager {
				log.WithFields(f).Debugf("Adding CLA Manager permissions...")

				hasScope, scopeErr := orgClient.IsUserHaveRoleScope(ctx, utils.CLAManagerRole, lfUser.ID, org.ID, foundationSFID)
				if scopeErr != nil {
					log.WithFields(f).WithError(scopeErr).Warnf("Failed to get scope for role: %s , user:%s , foundation: %s and org: %s ", utils.CLAManagerRole, lfUser.ID, foundationSFID, org.ID)
					continue
				}
				if hasScope {
					log.WithFields(f).Warnf("User: %s already has scope ", mgr)
					continue
				}
				createErr := orgClient.CreateOrgUserRoleOrgScopeProjectOrg(ctx, *email, foundationSFID, org.ID, roleID)
				if createErr != nil {
					log.WithFields(f).WithError(createErr).Warnf("failed to create CLA Manager role for user: %s ", *email)
					return scopeErr
				}

			} else if action == DeleteCLAManager {
				log.WithFields(f).Debugf("Deleting CLA Manager permissions...")
				deleteErr := orgClient.DeleteOrgUserRoleOrgScopeProjectOrg(ctx, org.ID, roleID, scopeID, &mgr, email)
				if deleteErr != nil {
					log.WithFields(f).WithError(deleteErr).Warn("Failed to remove CLA Manager from user: &s ", mgr)
					return deleteErr
				}
			}
		} else {
			if action == AddCLAManager {
				log.WithFields(f).Debugf("Adding CLA Manager permissions...")
				var eg errgroup.Group
				// add user as cla-manager for all projects of cla-group
				for _, projectSfid := range projectSFIDList.List() {
					// ensure that following goroutine gets a copy of projectSFID
					projectSFID := projectSfid
					eg.Go(func() error {
						log.WithFields(f).Debugf("Adding CLA Manager role...")
						err := orgClient.CreateOrgUserRoleOrgScopeProjectOrg(ctx, *email, projectSFID, org.ID, roleID)
						if err != nil {
							msg := fmt.Sprintf("unable to add %s scope for project: %s, company: %s using roleID: %s for user email: %s error = %s",
								utils.CLAManagerRole, projectSFID, org.ID, roleID, *email, err)
							log.WithFields(f).Warn(msg)
							return err
						}
						return nil
					})
				}
				// Wait for the go routines to finish
				log.WithFields(f).Debugf("waiting for create role assignment to complete for %d projects...", len(projectSFIDList.List()))
				if loadErr := eg.Wait(); loadErr != nil {
					log.WithFields(f).Warn(loadErr)
					continue
				}
			} else if action == DeleteCLAManager {
				log.WithFields(f).Debugf("Deleting CLA Manager permissions...")
				var eg errgroup.Group
				// remove user as cla-manager for all projects of cla-group
				for _, projectSfid := range projectSFIDList.List() {
					// ensure that following goroutine gets a copy of projectSFID
					projectSFID := projectSfid
					eg.Go(func() error {
						scopeID, scopeErr := orgClient.GetScopeID(ctx, org.ID, projectSFID, utils.CLAManagerRole, utils.ProjectOrgScope, mgr)
						if scopeErr != nil {
							log.WithFields(f).Warn(scopeErr)
							return scopeErr
						}
						log.WithFields(f).Debugf("Removing CLA Manager role...")
						deleteErr := orgClient.DeleteOrgUserRoleOrgScopeProjectOrg(ctx, org.ID, roleID, scopeID, &mgr, email)
						if deleteErr != nil {
							log.WithFields(f).Warn(deleteErr)
							return deleteErr
						}
						return nil
					})
				}
				// Wait for the go routines to finish
				log.WithFields(f).Debugf("waiting for delete role assignment to complete for %d projects...", len(projectSFIDList.List()))
				if loadErr := eg.Wait(); loadErr != nil {
					log.WithFields(f).Warn(loadErr)
					continue
				}
			}
		}
	}
	return nil

}
