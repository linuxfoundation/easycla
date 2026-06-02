// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"context"
	"testing"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	orgModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/sss"
	"github.com/sirupsen/logrus"
)

func TestResolveDomainPrefersDomains(t *testing.T) {
	svc := &service{}

	got := svc.resolveDomain(logrus.Fields{}, &orgModels.Organization{
		Domains: []string{"www.example.com", "fallback.example.org"},
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
	}

	if _, ok := svc.getComplianceCache("company-id"); ok {
		t.Fatal("expected expired cache entry to be ignored")
	}
}

func TestComplianceCacheSkipsErrors(t *testing.T) {
	svc := &service{}

	// setComplianceCache no longer takes an err param; just verify it stores the entry
	svc.setComplianceCache("company-id", false)

	if _, ok := svc.getComplianceCache("company-id"); !ok {
		t.Fatal("expected cache entry to be stored")
	}
}
