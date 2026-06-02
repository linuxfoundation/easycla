// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"errors"
	"testing"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/sss"
	"github.com/sirupsen/logrus"
)

type testOrg struct {
	domains []string
	link    string
}

func (o testOrg) GetDomains() []string {
	return o.domains
}

func (o testOrg) GetLink() string {
	return o.link
}

func TestResolveDomainPrefersDomains(t *testing.T) {
	svc := &service{}

	got := svc.resolveDomain(logrus.Fields{}, testOrg{
		domains: []string{"www.example.com"},
		link:    "https://fallback.example.org/path",
	})

	if got != "example.com" {
		t.Fatalf("expected domain from Domains field, got %q", got)
	}
}

func TestResolveDomainFallsBackToParsedLink(t *testing.T) {
	svc := &service{}

	got := svc.resolveDomain(logrus.Fields{}, testOrg{
		link: "www.example.org/path?query=1",
	})

	if got != "example.org" {
		t.Fatalf("expected parsed Link hostname, got %q", got)
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
				err:        errors.New("cached"),
				expiresAt:  time.Now().Add(-time.Second),
			},
		},
	}

	if _, ok := svc.getComplianceCache("company-id"); ok {
		t.Fatal("expected expired cache entry to be ignored")
	}
}
