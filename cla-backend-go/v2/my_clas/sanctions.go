// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"errors"
	"net/url"
	"strings"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/sss"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	organizationService "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
	orgModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/models"
	"github.com/sirupsen/logrus"
)

const sanctionOriginSSS = "sss"

// SanctionsScreener answers the sanctions question for an employer. It never errors: a listing
// must not fail because screening is down, so an unusable screen degrades to the persisted company
// flag and reports check=unavailable.
type SanctionsScreener interface {
	Mode() string
	ScreenCompany(ctx context.Context, company *v1Models.Company) (flagged bool, check string)
}

type sssStatusClient interface {
	GetOrganizationStatus(ctx context.Context, req sss.OrganizationStatusRequest) (*sss.ScreeningResult, error)
}

type sssScreener struct {
	client          sssStatusClient
	enabled         bool
	required        bool
	getOrganization func(ctx context.Context, orgID string) (*orgModels.Organization, error)
}

// NewSanctionsScreener returns a read-only screener: it answers the question and never writes.
// Persisting a first live detection is the caller's job (see service.persistLiveSanction)
func NewSanctionsScreener(client *sss.Client, enabled, required bool) SanctionsScreener {
	screener := &sssScreener{
		enabled:         enabled,
		required:        required,
		getOrganization: lookupOrganization,
	}
	if client != nil {
		screener.client = client
	}
	return screener
}

func lookupOrganization(ctx context.Context, orgID string) (*orgModels.Organization, error) {
	client := organizationService.GetClient()
	if client == nil {
		return nil, errors.New("organization service client is not configured")
	}
	return client.GetOrganization(ctx, orgID)
}

func (s *sssScreener) Mode() string {
	if !s.enabled || s.client == nil {
		return models.MyClaListSssModeDisabled
	}
	if s.required {
		return models.MyClaListSssModeRequired
	}
	return models.MyClaListSssModeOptional
}

func (s *sssScreener) ScreenCompany(ctx context.Context, company *v1Models.Company) (bool, string) {
	if company == nil {
		return false, ""
	}

	f := logrus.Fields{
		"functionName":   "v2.my_clas.sanctions.ScreenCompany",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"companyID":      company.CompanyID,
		"mode":           s.Mode(),
	}

	// An administrator-set block is authoritative and needs no live screen (as in v2/sign
	// checkCompanyCompliance)
	if company.IsSanctioned && company.SanctionOrigin != sanctionOriginSSS {
		return true, models.MyClaFlaggedCheckStored
	}
	if !s.enabled || s.client == nil {
		return company.IsSanctioned, models.MyClaFlaggedCheckStored
	}

	unavailable := func(reason string) (bool, string) {
		log.WithFields(f).Warnf("live sanctions screening unavailable, honoring the persisted flag: %s", reason)
		return company.IsSanctioned, models.MyClaFlaggedCheckUnavailable
	}

	externalID := strings.TrimSpace(company.CompanyExternalID)
	if externalID == "" {
		return unavailable("company has no external ID for domain resolution")
	}
	org, err := s.getOrganization(ctx, externalID)
	if err != nil {
		return unavailable("organization lookup failed: " + err.Error())
	}
	if org == nil {
		return unavailable("organization record is nil for " + externalID)
	}
	domain := resolveOrgDomain(org)
	if domain == "" {
		return unavailable("unable to resolve a domain for organization " + externalID)
	}

	req := sss.OrganizationStatusRequest{Domain: domain, OrgName: company.CompanyName}
	if strings.HasPrefix(externalID, "001") {
		req.SFDCID = externalID
	}
	result, err := s.client.GetOrganizationStatus(ctx, req)
	if err != nil {
		return unavailable("SSS call failed: " + err.Error())
	}
	if result == nil || (result.Status != sss.StatusClean && result.Status != sss.StatusFlagged) {
		return unavailable("unexpected SSS status")
	}
	return result.Status == sss.StatusFlagged, models.MyClaFlaggedCheckLive
}

// resolveOrgDomain prefers the Domains field and falls back to the host of Link
func resolveOrgDomain(org *orgModels.Organization) string {
	for _, domain := range strings.Split(org.Domains, ",") {
		if domain = strings.TrimPrefix(strings.TrimSpace(domain), "www."); domain != "" {
			return domain
		}
	}
	link := strings.TrimSpace(org.Link)
	if link == "" {
		return ""
	}
	if !strings.Contains(link, "://") {
		link = "https://" + link
	}
	parsed, err := url.Parse(link)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(parsed.Hostname(), "www.")
}
