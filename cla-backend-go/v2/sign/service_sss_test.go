// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/sss"
	orgModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/models"
	"github.com/sirupsen/logrus"
)

func TestResolveDomainPrefersDomains(t *testing.T) {
	svc := &service{}

	got := svc.resolveDomain(logrus.Fields{}, &orgModels.Organization{
		Domains: "www.example.com,fallback.example.org",
		Link:    "https://fallback.example.org/path",
	})

	if got != "example.com" {
		t.Fatalf("expected domain from Domains field, got %q", got)
	}
}

func TestResolveDomainFallsBackToParsedLink(t *testing.T) {
	svc := &service{}

	got := svc.resolveDomain(logrus.Fields{}, &orgModels.Organization{
		Link: "www.example.org/path?query=1",
	})

	if got != "example.org" {
		t.Fatalf("expected parsed Link hostname, got %q", got)
	}
}

func TestCheckCompanyComplianceRequiredBlocksMissingClient(t *testing.T) {
	svc := &service{sssRequired: true}

	_, err := svc.checkCompanyCompliance(context.Background(), &models.Company{
		CompanyID:   "company-id",
		CompanyName: "Company",
	})
	if err == nil {
		t.Fatal("expected required SSS missing client to block")
	}
}

func TestCheckCompanyComplianceOptionalAllowsMissingClient(t *testing.T) {
	svc := &service{sssRequired: false}

	blocked, err := svc.checkCompanyCompliance(context.Background(), &models.Company{
		CompanyID:   "company-id",
		CompanyName: "Company",
	})
	if err != nil {
		t.Fatalf("expected optional SSS missing client to continue, got %v", err)
	}
	if blocked {
		t.Fatal("expected optional SSS missing client not to block")
	}
}

func newTestSSSClient(t *testing.T) *sss.Client {
	t.Helper()
	client, err := sss.NewClient(sss.SSSConfig{
		BaseURL:           "https://sss.example.com",
		Auth0Domain:       "example.auth0.com",
		Auth0ClientID:     "client-id",
		Auth0ClientSecret: "client-secret",
		Auth0Audience:     "https://sss.example.com/",
	})
	if err != nil {
		t.Fatalf("failed to build SSS client: %v", err)
	}
	return client
}

func TestCheckCompanyComplianceRequiredBlocksMissingExternalID(t *testing.T) {
	svc := &service{sssRequired: true, sssClient: newTestSSSClient(t)}

	_, err := svc.checkCompanyCompliance(context.Background(), &models.Company{
		CompanyID:   "company-id",
		CompanyName: "Company",
		// CompanyExternalID intentionally empty
	})
	if err == nil {
		t.Fatal("expected required SSS to block a company with no external ID")
	}
}

func TestCheckCompanyComplianceOptionalAllowsMissingExternalID(t *testing.T) {
	svc := &service{sssRequired: false, sssClient: newTestSSSClient(t)}

	blocked, err := svc.checkCompanyCompliance(context.Background(), &models.Company{
		CompanyID:   "company-id",
		CompanyName: "Company",
		// CompanyExternalID intentionally empty; not persisted as sanctioned
	})
	if err != nil {
		t.Fatalf("expected optional SSS to continue when external ID is missing, got %v", err)
	}
	if blocked {
		t.Fatal("expected optional SSS not to block when external ID is missing")
	}
}

func TestHandleSSSErrorRequiredBlocksAvailabilityErrors(t *testing.T) {
	svc := &service{sssRequired: true}

	_, err := svc.handleSSSError(logrus.Fields{}, "company-id", &sss.RetryableError{Message: "unavailable"})
	if err == nil {
		t.Fatal("expected required SSS retryable error to block")
	}
}

func TestHandleSSSErrorOptionalAllowsAuthErrors(t *testing.T) {
	svc := &service{sssRequired: false}

	blocked, err := svc.handleSSSError(logrus.Fields{}, "company-id", &sss.AuthError{Message: "auth failed"})
	if err != nil {
		t.Fatalf("expected optional SSS auth error to continue, got %v", err)
	}
	if blocked {
		t.Fatal("expected optional SSS auth error not to block")
	}
}

func TestHandleSSSErrorOptionalAllowsBadRequest(t *testing.T) {
	svc := &service{sssRequired: false}

	blocked, err := svc.handleSSSError(logrus.Fields{}, "company-id", &sss.BadRequestError{Message: "bad request"})
	if err != nil {
		t.Fatalf("expected optional SSS bad request to continue, got %v", err)
	}
	if blocked {
		t.Fatal("expected optional SSS bad request not to block")
	}
}

func TestComplianceCacheKeyPrefersExternalID(t *testing.T) {
	svc := &service{}

	got := svc.complianceCacheKey(&models.Company{
		CompanyID:         "internal-id",
		CompanyExternalID: "external-id",
	})

	if got != "external-id" {
		t.Fatalf("expected external id cache key, got %q", got)
	}
}

func TestComplianceCacheExpires(t *testing.T) {
	svc := &service{
		complianceCache: map[string]complianceCacheEntry{
			"company-id": {
				sanctioned: true,
				expiresAt:  time.Now().Add(-time.Second),
			},
		},
		complianceCacheMu: &sync.Mutex{},
	}

	if _, ok := svc.getComplianceCache("company-id"); ok {
		t.Fatal("expected expired cache entry to be ignored")
	}
}

func TestComplianceCacheSkipsErrors(t *testing.T) {
	svc := &service{complianceCacheMu: &sync.Mutex{}}

	// setComplianceCache no longer takes an err param; just verify it stores the entry
	svc.setComplianceCache("company-id", false)

	if _, ok := svc.getComplianceCache("company-id"); !ok {
		t.Fatal("expected cache entry to be stored")
	}
}

func TestCheckCompanyComplianceCacheHitMutatesModel(t *testing.T) {
	// A cached "clean" result must clear the stale loaded model so downstream gates
	// (e.g. ProcessEmployeeSignature) in the same request stay consistent.
	svc := &service{
		complianceCache: map[string]complianceCacheEntry{
			"external-id": {sanctioned: false, expiresAt: time.Now().Add(time.Minute)},
		},
		complianceCacheMu: &sync.Mutex{},
	}

	company := &models.Company{
		CompanyID:         "company-id",
		CompanyExternalID: "external-id",
		IsSanctioned:      true,
		SanctionOrigin:    "sss",
	}

	blocked, err := svc.checkCompanyCompliance(context.Background(), company)
	if err != nil {
		t.Fatalf("expected cached clean result to continue, got %v", err)
	}
	if blocked {
		t.Fatal("expected cached clean result not to block")
	}
	if company.IsSanctioned || company.SanctionOrigin != "" {
		t.Fatalf("expected in-memory model cleared on cache hit, got IsSanctioned=%v origin=%q", company.IsSanctioned, company.SanctionOrigin)
	}
}

func TestCheckCompanyComplianceOptionalHonorsPersistedSSSFlag(t *testing.T) {
	// Optional mode with no SSS client: an already-persisted SSS-origin block must keep
	// blocking until a live clean result can clear it (do not fail open on the flag).
	svc := &service{sssRequired: false}

	blocked, err := svc.checkCompanyCompliance(context.Background(), &models.Company{
		CompanyID:      "company-id",
		CompanyName:    "Company",
		IsSanctioned:   true,
		SanctionOrigin: "sss",
	})
	if err != nil {
		t.Fatalf("expected optional persisted-sanction check to continue without error, got %v", err)
	}
	if !blocked {
		t.Fatal("expected a persisted SSS sanction to keep blocking in optional mode when SSS is unavailable")
	}
}

func TestCheckCompanyComplianceAdminBlockAlwaysBlocks(t *testing.T) {
	// A manual/admin block (is_sanctioned=true, no/!=sss origin) must short-circuit and
	// block regardless of mode or SSS availability.
	svc := &service{sssRequired: false}

	blocked, err := svc.checkCompanyCompliance(context.Background(), &models.Company{
		CompanyID:    "company-id",
		IsSanctioned: true,
	})
	if err != nil {
		t.Fatalf("unexpected error for admin block: %v", err)
	}
	if !blocked {
		t.Fatal("expected a manual/admin sanction (no origin) to always block")
	}
}
