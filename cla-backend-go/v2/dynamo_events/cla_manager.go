// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	v2AcsService "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	v2OrgService "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
	v2UserService "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service"
	"github.com/sirupsen/logrus"
)

// SetInitialCLAManagerACSPermissions establishes the initial CLA manager permissions
func (s *service) SetInitialCLAManagerACSPermissions(ctx context.Context, signatureID string) error {
	f := logrus.Fields{
		"functionName":   "SetInitialCLAManagerACSPermissions",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"signatureID":    signatureID,
	}

	sig, err := s.signatureRepo.GetSignature(ctx, signatureID)
	if err != nil {
		log.WithFields(f).Warnf("problem locating signature by ID, error: %+v", err)
		return err
	}

	f["signatureType"] = sig.SignatureType
	f["referenceID"] = sig.SignatureReferenceID
	f["referenceName"] = sig.SignatureReferenceName
	f["referenceType"] = sig.SignatureReferenceType
	f["projectID"] = sig.ProjectID
	f["signed"] = sig.SignatureSigned
	f["approved"] = sig.SignatureApproved
	f["embargo_acked"] = sig.SignatureEmbargoAcked
	f["companyName"] = sig.CompanyName
	f["claType"] = sig.ClaType

	if sig.SignatureType != CCLASignatureType {
		log.WithFields(f).Warn("invalid signature type for setting initial cla manager request")
		return fmt.Errorf("invalid signature type for setting initial cla manager request. %s", signatureID)
	}
	if len(sig.SignatureACL) == 0 {
		log.WithFields(f).Warn("initial cla manager details not found")
		return errors.New("initial cla manager details not found")
	}

	if len(sig.SignatureACL) > 1 {
		log.WithFields(f).Warnf("%d initial cla managers specified in the signature record - this likely a mistake.", len(sig.SignatureACL))
	}

	// get user details
	userServiceClient := v2UserService.GetClient()
	log.WithFields(f).Debugf("searching user by username: %s", sig.SignatureACL[0].LfUsername)
	claManager, err := userServiceClient.GetUserByUsername(sig.SignatureACL[0].LfUsername)
	// Find it? If not, we'll try a couple of approaches before giving up...
	if err != nil || claManager == nil {
		log.WithFields(f).Warnf("unable to lookup user by username: %s, error: %+v",
			sig.SignatureACL[0].LfUsername, err)

		log.WithFields(f).Debugf("searching user by email: %s", sig.SignatureACL[0].LfEmail)
		if sig.SignatureACL[0].LfEmail != "" {
			claManager, err = userServiceClient.SearchUsersByEmail(sig.SignatureACL[0].LfEmail.String())
			if err != nil || claManager == nil {
				log.WithFields(f).Warnf("unable to lookup user by email: %s, error: %+v",
					sig.SignatureACL[0].LfEmail, err)
			}
		}

		// Haven't found it yet - do we have any alternative emails?
		if claManager == nil && sig.SignatureACL[0].Emails != nil {
			// Search each one...
			for _, altEmail := range sig.SignatureACL[0].Emails {
				log.WithFields(f).Debugf("searching user by alternate email: %s", altEmail)
				claManager, err = userServiceClient.SearchUsersByEmail(altEmail)
				if err != nil || claManager == nil {
					log.WithFields(f).Warnf("unable to lookup user by alternate email: %s, error: %+v",
						altEmail, err)
				}

				// Found it!
				if claManager != nil {
					break
				}
			}
		}

		// Bummer, didn't find it... time to bail out...
		if claManager == nil {
			if err == nil {
				msg := "unable to locate user using username, lf email or alternate emails - giving up"
				log.WithFields(f).Warn(msg)
				err = errors.New(msg)
			}
			return err
		}

		// Fall through - we have a valid claManager record!!
	}
	log.WithFields(f).Debugf("found user: %+v", claManager)

	// get the preferred email from the user details
	var email string
	for _, e := range claManager.Emails {
		if e != nil && e.IsPrimary != nil && *e.IsPrimary {
			email = utils.StringValue(e.EmailAddress)
		}
	}

	log.WithFields(f).Debug("locating company record by signature reference ID...")
	company, err := s.companyRepo.GetCompany(ctx, sig.SignatureReferenceID)
	if err != nil {
		log.WithFields(f).Warnf("unable to lookup company by signature reference ID: %s, error: %+v",
			sig.SignatureReferenceID, err)
		return err
	}

	// fetch list of projects under cla group
	log.WithFields(f).Debug("locating SF projects associated with the CLA Group...")
	projectList, err := s.projectsClaGroupRepo.GetProjectsIdsForClaGroup(ctx, sig.ProjectID)
	if err != nil {
		log.WithFields(f).Warnf("unable to fetch list of projects associated with CLA Group: %s, error: %+v",
			sig.ProjectID, err)
		return err
	}
	log.WithFields(f).Debugf("discovered %d SF projects associated with the CLA Group...", len(projectList))

	// Build a quick string for the output log
	var projectInfoMsg strings.Builder
	for _, project := range projectList {
		projectInfoMsg.WriteString(project.ProjectName + "(" + project.ProjectSFID + "), ")
	}

	// Assign cla manager role based on level
	log.WithFields(f).Debugf("assigning %s role for projects: %s", utils.CLAManagerRole, projectInfoMsg.String())
	err = s.assignCLAManager(ctx, email, claManager.Username, company.CompanyExternalID, projectList)
	if err != nil {
		log.WithFields(f).Warnf("unable to assign CLA Manager %s for company: %s, error: %+v",
			claManager.Username, company.CompanyExternalID, err)
		return err
	}

	return nil
}

func (s service) assignCLAManager(ctx context.Context, email, username, companySFID string, projectList []*projects_cla_groups.ProjectClaGroup) error {
	f := logrus.Fields{
		"functionName":   "assignCLAManager",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"email":          email,
		"username":       username,
		"companySFID":    companySFID,
	}

	if len(projectList) == 0 {
		msg := fmt.Sprintf("Unable to assign %s role to user: %s with email: %s - no projects specified",
			utils.CLAManagerRole, username, email)
		log.WithFields(f).Warn(msg)
		return errors.New(msg)
	}

	// // check if project is signed at foundation level
	// foundationID := projectList[0].FoundationSFID
	// f["foundationID"] = projectList[0].FoundationSFID
	// log.WithFields(f).Debugf("using first project's foundation ID: %s", foundationID)

	// log.WithFields(f).Debugf("determining if this project happens to be signed at the foundation level, foundationID: %s", foundationID)
	// signedAtFoundation, signedErr := s.projectService.SignedAtFoundationLevel(ctx, foundationID)
	// if signedErr != nil {
	// 	return signedErr
	// }

	acsClient := v2AcsService.GetClient()
	log.WithFields(f).Debugf("locating role ID for role: %s", utils.CLAManagerRole)
	claManagerRoleID, roleErr := acsClient.GetRoleID(utils.CLAManagerRole)
	if roleErr != nil {
		log.WithFields(f).Warnf("problem looking up details for role: %s, error: %+v", utils.CLAManagerRole, roleErr)
		return roleErr
	}

	orgService := v2OrgService.GetClient()

	projectSFIDList := utils.NewStringSet()
	for _, p := range projectList {
		projectSFIDList.Add(p.ProjectSFID)
	}

	var assignErr error
	var wg sync.WaitGroup
	wg.Add(len(projectSFIDList.List()))

	// add user as cla-manager for all projects of cla-group
	for _, projectSFID := range projectSFIDList.List() {
		go func(projectSFID string) {
			defer wg.Done()
			log.WithFields(f).Debugf("assigning role: %s to user: %s with email: %s for project: %s", utils.CLAManagerRole, username, email, projectSFID)
			err := orgService.CreateOrgUserRoleOrgScopeProjectOrg(ctx, email, projectSFID, companySFID, claManagerRoleID)
			if err != nil {
				log.WithFields(f).Warnf("unable to add %s scope for project: %s, company: %s using roleID: %s for user email: %s. error = %s",
					utils.CLAManagerRole, projectSFID, companySFID, claManagerRoleID, email, err)
				if err != nil {
					assignErr = err
				}
			}
		}(projectSFID)
	}

	wg.Wait()

	if assignErr != nil {
		return assignErr
	}

	return nil
}
