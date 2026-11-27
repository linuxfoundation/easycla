// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"context"
	"net/url"
	"regexp"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

var orgLensAccountIDPattern = regexp.MustCompile(`^001[A-Za-z0-9]{15}$`)

var orgLensLiveCheck func(ctx context.Context, companySFID string) bool

// SetOrgLensLiveCheck sets the check that a company's Salesforce Account is live; until it is set no company gets an Organization lens address
func SetOrgLensLiveCheck(check func(ctx context.Context, companySFID string) bool) {
	orgLensLiveCheck = check
}

// OrgLensURL returns the Organization lens address for a company's CLA Group page,
// /org/{companySFID}/easycla/{claGroupID}. Without a CLA Group it is the company's EasyCLA page.
// It returns "" when base is empty or unparseable, or when companySFID is not the 18-character
// ID of a live Salesforce Account, which callers treat as "use the Corporate Console".
func OrgLensURL(base, companySFID, claGroupID string) string {
	return OrgLensURLContext(context.Background(), base, companySFID, claGroupID)
}

// OrgLensURLContext is OrgLensURL with the caller's context for the live Account check.
func OrgLensURLContext(ctx context.Context, base, companySFID, claGroupID string) string {
	base = strings.TrimSpace(base)
	companySFID = strings.TrimSpace(companySFID)
	if base == "" || !orgLensAccountIDPattern.MatchString(companySFID) {
		return ""
	}
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		log.Warnf("unable to parse the Organization lens base URL %q: %v", base, err)
		return ""
	}
	if canonical, ok := member_service.SFID18(companySFID); ok {
		companySFID = canonical
	}
	if orgLensLiveCheck == nil || !orgLensLiveCheck(ctx, companySFID) {
		return ""
	}

	claGroupID = strings.TrimSpace(claGroupID)
	segments := []string{"org", companySFID, "easycla"}
	if claGroupID != "" {
		segments = append(segments, claGroupID)
	}
	basePath := strings.TrimSuffix(u.EscapedPath(), "/")
	decoded := []string{strings.TrimSuffix(u.Path, "/")}
	escaped := []string{basePath}
	for _, s := range segments {
		decoded = append(decoded, s)
		escaped = append(escaped, url.PathEscape(s))
	}
	u.Path = strings.Join(decoded, "/")
	u.RawPath = strings.Join(escaped, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// GetOrgLensURL returns OrgLensURL for the configured Organization lens base,
// cla-org-lens-base-{stage}.
func GetOrgLensURL(companySFID, claGroupID string) string {
	return OrgLensURL(config.GetConfig().OrgLensURL, companySFID, claGroupID)
}

// GetOrgLensURLContext is GetOrgLensURL with the caller's context for the live Account check.
func GetOrgLensURLContext(ctx context.Context, companySFID, claGroupID string) string {
	return OrgLensURLContext(ctx, config.GetConfig().OrgLensURL, companySFID, claGroupID)
}
