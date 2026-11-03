// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package utils

import (
	"net/url"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

// OrgLensURL returns the Organization lens address for a company's CLA Group page,
// /org/{companySFID}/easycla/{claGroupID}. Without a CLA Group it is the company's EasyCLA page,
// and without a company SFID it is the lens home, where the viewer picks the organization.
// It returns "" when base is empty or unparseable, which callers treat as "no lens configured".
func OrgLensURL(base, companySFID, claGroupID string) string {
	base = strings.TrimSpace(base)
	if base == "" {
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

	companySFID = strings.TrimSpace(companySFID)
	claGroupID = strings.TrimSpace(claGroupID)
	segments := []string{"org"}
	if companySFID != "" {
		segments = append(segments, companySFID, "easycla")
		if claGroupID != "" {
			segments = append(segments, claGroupID)
		}
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

// GetOrgLensURL returns OrgLensURL for the configured Organization lens base, or "" when
// cla-org-lens-base-{stage} is not provisioned.
func GetOrgLensURL(companySFID, claGroupID string) string {
	return OrgLensURL(config.GetConfig().OrgLensURL, companySFID, claGroupID)
}
