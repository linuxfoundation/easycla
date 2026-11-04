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

// ErrCompanyNotEligible reports a Salesforce account that member-service refuses to register as an organization
var ErrCompanyNotEligible = errors.New("the organization is not eligible for EasyCLA")

// ErrCompanyDependencyFailed reports a member-service, repository, Organization Service or ACS failure - not a caller error
var ErrCompanyDependencyFailed = errors.New("company creation is temporarily unavailable")

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
// EasyCLA company on its 001 id; only a created account grants company-admin to the creator after the row exists.
func (s *service) createCompanyViaMemberService(ctx context.Context, f logrus.Fields, companyName, signingEntityName, companyWebsite, userEmail, userID, note string, lfUser *v2UserServiceModels.User) (*models.CompanyOutput, error) {
	f = memberServiceLogFields(f)
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
	if takenErr := s.companyNameTaken(ctx, f, companyName); takenErr != nil {
		return nil, takenErr
	}
	b2b, action, err := s.members.FindOrCreateB2BOrg(ctx, companyName, companyWebsite)
	if err != nil {
		switch {
		case errors.Is(err, memberService.ErrAmbiguousDomain):
			log.WithFields(f).WithError(err).Warn("several Salesforce accounts match the website domain - refusing to create the company")
			return nil, fmt.Errorf("%w: %s", ErrCompanyAmbiguousDomain, companyWebsite)
		case errors.Is(err, memberService.ErrNotEligible):
			log.WithFields(f).WithError(err).Warn("member-service refused to register the account as an organization")
			return nil, fmt.Errorf("%w: %v", ErrCompanyNotEligible, err)
		case errors.Is(err, memberService.ErrInvalidRequest):
			log.WithFields(f).WithError(err).Warn("member-service rejected the company request")
			return nil, err
		default:
			log.WithFields(f).WithError(err).Warn("member-service find-or-create failed")
			return nil, fmt.Errorf("%w: %v", ErrCompanyDependencyFailed, err)
		}
	}
	f["companySFID"], f["memberServiceAction"] = b2b.UID, action
	if b2b.Name != "" && b2b.Name != companyName {
		log.WithFields(f).Debugf("the Salesforce account name differs from the request - using %s", b2b.Name)
		companyName = b2b.Name
		if takenErr := s.companyNameTaken(ctx, f, companyName); takenErr != nil {
			return nil, takenErr
		}
	}
	if signingEntityName == "" {
		signingEntityName = companyName
	}
	if action == memberService.ActionMatched {
		existing, lookupErr := s.companyBySFID(ctx, b2b.UID)
		if lookupErr != nil {
			log.WithFields(f).WithError(lookupErr).Warn("unable to check whether the matched account is already an EasyCLA company")
			return nil, fmt.Errorf("%w: %v", ErrCompanyDependencyFailed, lookupErr)
		}
		if existing != nil {
			log.WithFields(f).Warnf("the matched account is already EasyCLA company %s - refusing to create another", existing.CompanyID)
			return nil, fmt.Errorf("%w: %s", ErrCompanyExists, b2b.UID)
		}
	}
	row := &v1Models.Company{
		CompanyExternalID: b2b.UID,
		CompanyName:       companyName,
		SigningEntityName: signingEntityName,
		CompanyACL:        []string{},
		Note:              note,
		IsSanctioned:      false,
	}
	if action == memberService.ActionCreated {
		row.CompanyManagerID = userID
		row.CompanyACL = []string{lfUser.Username}
	}
	created, createErr := s.companyRepo.CreateCompany(ctx, row)
	if createErr != nil {
		log.WithFields(f).WithError(createErr).Warnf("failed to create EasyCLA company %s", companyName)
		return nil, fmt.Errorf("%w: %v", ErrCompanyDependencyFailed, createErr)
	}
	if created == nil || created.CompanyExternalID != b2b.UID {
		log.WithFields(f).Warnf("an EasyCLA company named %q already exists with a different Salesforce account - refusing to create another", companyName)
		return nil, fmt.Errorf("%w: %s", ErrCompanyExists, companyName)
	}
	// the creator-owned row exists before the grant, so a failed grant leaves an owned row for the retry, not a gap
	if action == memberService.ActionCreated {
		if grantErr := s.grantCreatorIfOrgKnown(ctx, f, b2b.UID, userEmail, userID, lfUser.Username); grantErr != nil {
			return nil, grantErr
		}
	}
	return &models.CompanyOutput{
		CompanyName:       created.CompanyName,
		SigningEntityName: created.SigningEntityName,
		CompanyWebsite:    companyWebsite,
		CompanyID:         created.CompanyExternalID,
	}, nil
}

// memberServiceLogFields copies the caller's fields without the e-mail address: this path logs the user id only
func memberServiceLogFields(f logrus.Fields) logrus.Fields {
	out := make(logrus.Fields, len(f))
	for k, v := range f {
		if k != "userEmail" {
			out[k] = v
		}
	}
	return out
}

// companyNameTaken answers ErrCompanyExists when an EasyCLA company already carries the name (the repository would
// otherwise silently return that row), before any Salesforce write or grant
func (s *service) companyNameTaken(ctx context.Context, f logrus.Fields, companyName string) error {
	existing, err := s.companyRepo.GetCompanyByName(ctx, companyName)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to check whether the company name is already taken")
		return fmt.Errorf("%w: %v", ErrCompanyDependencyFailed, err)
	}
	if existing != nil {
		log.WithFields(f).Warnf("an EasyCLA company named %q already exists - refusing to create another", companyName)
		return fmt.Errorf("%w: %s", ErrCompanyExists, companyName)
	}
	return nil
}

// companyBySFID finds the EasyCLA company recorded for the account: the canonical 18-character id first, then the
// 15-character form a legacy row may carry; nil when neither is recorded
func (s *service) companyBySFID(ctx context.Context, sfid string) (*v1Models.Company, error) {
	var notFound *utils.CompanyNotFound
	existing, err := s.companyRepo.GetCompanyByExternalID(ctx, sfid)
	if err == nil || !errors.As(err, &notFound) || len(sfid) != 18 {
		return existing, err
	}
	existing, err = s.companyRepo.GetCompanyByExternalID(ctx, sfid[:15])
	if err != nil && errors.As(err, &notFound) {
		return nil, nil
	}
	return existing, err
}

// grantCreatorIfOrgKnown skips the Organization Service readiness probe in ACS-direct mode.
func (s *service) grantCreatorIfOrgKnown(ctx context.Context, f logrus.Fields, sfid, userEmail, userID, username string) error {
	orgClient := orgService.GetClient()
	direct := acsService.DirectCLARolesEnabled()
	var orgErr error
	if !direct {
		_, orgErr = orgClient.GetOrganization(ctx, sfid)
	}
	switch {
	case orgErr == nil:
		roleID, adminErr := acsService.GetClient().GetRoleID(utils.CompanyAdminRole)
		if adminErr != nil {
			entry := log.WithFields(f)
			if !direct {
				entry = entry.WithError(adminErr)
			}
			entry.Warn("problem getting the company-admin role ID")
			return fmt.Errorf("%w: %v", ErrCompanyDependencyFailed, adminErr)
		}
		if scopeErr := orgClient.CreateCLAOrgUserRoleScope(ctx, userEmail, username, sfid, roleID); scopeErr != nil && (direct || !isOrgAssociated(scopeErr)) {
			entry := log.WithFields(f)
			if !direct {
				entry = entry.WithError(scopeErr)
			}
			entry.Warnf("problem creating the company-admin scope for user %s", userID)
			return fmt.Errorf("%w: %v", ErrCompanyDependencyFailed, scopeErr)
		}
		return nil
	case isOrgNotFound(orgErr):
		log.WithFields(f).Warnf("the Organization Service does not know account %s yet - creating the company without the company-admin grant for user %s", sfid, userID)
		return nil
	default:
		log.WithFields(f).WithError(orgErr).Warn("unable to read the account from the Organization Service")
		return fmt.Errorf("%w: %v", ErrCompanyDependencyFailed, orgErr)
	}
}

func isOrgNotFound(err error) bool {
	var notFound *organizations.GetOrgNotFound
	return errors.As(err, &notFound)
}
