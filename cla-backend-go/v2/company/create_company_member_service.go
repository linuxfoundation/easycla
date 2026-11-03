// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/orgimport"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	acsService "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	memberService "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
	orgService "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
	v2UserServiceModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
)

// ErrMemberServiceNotConfigured reports the create-company switch on without a usable member-service client
var ErrMemberServiceNotConfigured = errors.New("company creation through member-service is not configured")

// ErrCompanySharedDomain reports a website on a shared domain (public e-mail or hosting provider) that identifies no organization (lfx-self-serve#3794)
var ErrCompanySharedDomain = errors.New("the company website is on a shared domain")

// ErrCompanyAmbiguousDomain reports several Salesforce accounts sharing the website domain
var ErrCompanyAmbiguousDomain = errors.New("several organizations match the company website")

// ErrCompanyExists reports an EasyCLA company already recorded for the matched Account, or another Account under the same name
var ErrCompanyExists = errors.New("organization already exists")

var sharedDomains = defaultSharedDomains()

func defaultSharedDomains() orgimport.SharedDomains {
	set := orgimport.SharedDomains{}
	for _, d := range orgimport.DefaultSharedDomains {
		set[d] = true
	}
	return set
}

// EnableMemberServiceCreate routes company creation through member-service find-or-create (lfx-self-serve#4992);
// a nil client makes creation fail explicitly instead of falling back to the Organization Service
func (s *service) EnableMemberServiceCreate(client *memberService.Client) {
	s.createViaMemberService = true
	s.members = client
}

// createCompanyViaMemberService matches or creates the Salesforce account through member-service and creates the
// EasyCLA company on its 001 id; only a created account grants company-admin to the creator, and only while the
// Organization Service already knows it (lfx-self-serve#4992)
func (s *service) createCompanyViaMemberService(ctx context.Context, f logrus.Fields, companyName, signingEntityName, companyWebsite, userEmail, userID, note string, lfUser *v2UserServiceModels.User) (*models.CompanyOutput, error) {
	if s.members == nil {
		log.WithFields(f).Warn("the create-company switch is on but the member-service client is not configured - refusing to create the company")
		return nil, ErrMemberServiceNotConfigured
	}
	domain, gate := sharedDomains.Shared(companyWebsite)
	if gate != "" {
		log.WithFields(f).Warnf("website %q is %s - refusing to create the company", companyWebsite, gate)
		return nil, fmt.Errorf("%w: %s", ErrCompanySharedDomain, companyWebsite)
	}
	f["companyDomain"] = domain
	b2b, action, err := s.members.FindOrCreateB2BOrg(ctx, companyName, companyWebsite)
	if err != nil {
		if errors.Is(err, memberService.ErrAmbiguousDomain) {
			log.WithFields(f).WithError(err).Warn("several Salesforce accounts match the website domain - refusing to create the company")
			return nil, fmt.Errorf("%w: %s", ErrCompanyAmbiguousDomain, companyWebsite)
		}
		log.WithFields(f).WithError(err).Warn("member-service find-or-create failed")
		return nil, err
	}
	f["companySFID"], f["memberServiceAction"] = b2b.UID, action
	if b2b.Name != "" && b2b.Name != companyName {
		log.WithFields(f).Debugf("the Salesforce account name differs from the request - using %s", b2b.Name)
		companyName = b2b.Name
	}
	if signingEntityName == "" {
		signingEntityName = companyName
	}
	switch action {
	case memberService.ActionMatched:
		existing, lookupErr := s.companyRepo.GetCompanyByExternalID(ctx, b2b.UID)
		var notFound *utils.CompanyNotFound
		if lookupErr != nil && !errors.As(lookupErr, &notFound) {
			log.WithFields(f).WithError(lookupErr).Warn("unable to check whether the matched account is already an EasyCLA company")
			return nil, lookupErr
		}
		if existing != nil {
			log.WithFields(f).Warnf("the matched account is already EasyCLA company %s - refusing to create another", existing.CompanyID)
			return nil, fmt.Errorf("%w: %s", ErrCompanyExists, b2b.UID)
		}
	case memberService.ActionCreated:
		if grantErr := s.grantCreatorIfOrgKnown(ctx, f, b2b.UID, userEmail, lfUser.Username); grantErr != nil {
			return nil, grantErr
		}
	}
	row := &v1Models.Company{
		CompanyExternalID: b2b.UID,
		CompanyManagerID:  userID,
		CompanyName:       companyName,
		SigningEntityName: signingEntityName,
		CompanyACL:        []string{lfUser.Username},
		Note:              note,
		IsSanctioned:      false,
	}
	created, createErr := s.companyRepo.CreateCompany(ctx, row)
	if createErr != nil {
		log.WithFields(f).WithError(createErr).Warnf("failed to create EasyCLA company %s", companyName)
		return nil, createErr
	}
	if created == nil || created.CompanyExternalID != b2b.UID {
		log.WithFields(f).Warnf("an EasyCLA company named %q already exists with a different Salesforce account - refusing to create another", companyName)
		return nil, fmt.Errorf("%w: %s", ErrCompanyExists, companyName)
	}
	return &models.CompanyOutput{
		CompanyName:       created.CompanyName,
		SigningEntityName: created.SigningEntityName,
		CompanyWebsite:    companyWebsite,
		CompanyID:         created.CompanyExternalID,
	}, nil
}

// grantCreatorIfOrgKnown grants company-admin on a created account exactly as the Organization Service path does,
// unless the Organization Service does not know the account yet (propagation lag), which only skips the grant
func (s *service) grantCreatorIfOrgKnown(ctx context.Context, f logrus.Fields, sfid, userEmail, username string) error {
	orgClient := orgService.GetClient()
	_, orgErr := orgClient.GetOrganization(ctx, sfid)
	switch {
	case orgErr == nil:
		roleID, adminErr := acsService.GetClient().GetRoleID(utils.CompanyAdminRole)
		if adminErr != nil {
			log.WithFields(f).WithError(adminErr).Warn("problem getting the company-admin role ID")
			return adminErr
		}
		if scopeErr := orgClient.CreateOrgUserRoleOrgScope(ctx, userEmail, sfid, roleID); scopeErr != nil && !isOrgAssociated(scopeErr) {
			log.WithFields(f).WithError(scopeErr).Warnf("problem creating the company-admin scope for %s", userEmail)
			return scopeErr
		}
		return nil
	case isOrgNotFound(orgErr):
		log.WithFields(f).Warnf("the Organization Service does not know account %s yet - creating the company without the company-admin grant for %s", sfid, username)
		return nil
	default:
		log.WithFields(f).WithError(orgErr).Warn("unable to read the account from the Organization Service")
		return orgErr
	}
}

func isOrgNotFound(err error) bool {
	_, ok := err.(*organizations.GetOrgNotFound)
	return ok
}
