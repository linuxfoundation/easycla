// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package middleware

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

// CORS is a simple "always add CORS headers" middleware.
// The legacy Python backend sets these headers in response middleware.
var (
	allowedOriginsOnce sync.Once
	allowedOrigins     []string
	allowAllOrigins    bool
)

func normalizeAllowedOrigin(raw string) string {
	raw = strings.TrimSpace(strings.Trim(raw, "\"'"))
	if raw == "" || raw == "*" {
		return raw
	}

	// CLA_CONTRIBUTOR_BASE / CLA_CONTRIBUTOR_V2_BASE can be configured as a
	// hostname. Browser Origin values are scheme + host only.
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}

	return strings.TrimRight(raw, "/")
}

func addAllowedOrigin(raw string) {
	origin := normalizeAllowedOrigin(raw)
	if origin == "" {
		return
	}
	if origin == "*" {
		allowAllOrigins = true
		return
	}

	for _, existing := range allowedOrigins {
		if existing == origin {
			return
		}
	}
	allowedOrigins = append(allowedOrigins, origin)
}

func addContributorConsoleOrigins() {
	addAllowedOrigin(os.Getenv("CLA_CONTRIBUTOR_BASE"))
	addAllowedOrigin(os.Getenv("CLA_CONTRIBUTOR_V2_BASE"))
	addAllowedOrigin(os.Getenv("CLA_CORPORATE_BASE"))
	addAllowedOrigin(os.Getenv("CLA_CORPORATE_V2_BASE"))
	addAllowedOrigin(os.Getenv("CLA_LANDING_PAGE"))
}

func addBuiltInEasyCLAOrigins() {
	// Keep known EasyCLA dev/staging/prod UI and API aliases available even when
	// ALLOWED_ORIGINS in SSM is incomplete. Credentialed browser requests cannot
	// use a wildcard Access-Control-Allow-Origin value.
	for _, origin := range []string{
		"https://easycla.dev.communitybridge.org",
		"https://easycla.staging.communitybridge.org",
		"https://easycla.communitybridge.org",
		"https://easycla.dev.platform.linuxfoundation.org",
		"https://easycla.staging.platform.linuxfoundation.org",
		"https://easycla.lfx.linuxfoundation.org",
		"https://contributor.easycla.lfx.linuxfoundation.org",
		"https://project.dev.lfcla.com",
		"https://project.v1.easycla.lfx.linuxfoundation.org",
		"https://corporate.dev.lfcla.com",
		"https://corporate.v1.easycla.lfx.linuxfoundation.org",
		"https://api.lfcla.dev.platform.linuxfoundation.org",
		"https://api.lfcla.staging.platform.linuxfoundation.org",
		"https://api.easycla.lfx.linuxfoundation.org",
		"https://api.dev.lfcla.com",
		"https://api.staging.lfcla.com",
		"https://api.lfcla.com",
		"https://api-gw.dev.platform.linuxfoundation.org",
		"https://api-gw.staging.platform.linuxfoundation.org",
		"https://api-gw.platform.linuxfoundation.org",
	} {
		addAllowedOrigin(origin)
	}
}

func loadAllowedOriginsFromEnv() {
	raw := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS"))
	if raw == "" {
		// Backwards compatible default: allow all.
		allowAllOrigins = true
		addContributorConsoleOrigins()
		addBuiltInEasyCLAOrigins()
		return
	}

	// Supported formats:
	// - JSON array: ["https://a", "https://b"]
	// - CSV: https://a,https://b
	// - Space/newline separated
	if strings.HasPrefix(raw, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			for _, v := range arr {
				addAllowedOrigin(v)
			}
			addContributorConsoleOrigins()
			addBuiltInEasyCLAOrigins()
			if len(allowedOrigins) == 0 && !allowAllOrigins {
				allowAllOrigins = true
			}
			return
		}
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', ' ', '\n', '\t', '\r':
			return true
		default:
			return false
		}
	})
	for _, p := range parts {
		addAllowedOrigin(p)
	}

	// The legacy GitHub signing flow redirects contributors to these consoles.
	// Therefore these origins must be allowed to call the v1/v2 APIs after
	// GitHub OAuth redirects back to EasyCLA.
	addContributorConsoleOrigins()
	addBuiltInEasyCLAOrigins()

	if len(allowedOrigins) == 0 && !allowAllOrigins {
		allowAllOrigins = true
	}
}

func isOriginAllowed(origin string) bool {
	allowedOriginsOnce.Do(loadAllowedOriginsFromEnv)
	if allowAllOrigins {
		return true
	}

	origin = normalizeAllowedOrigin(origin)
	if origin == "" {
		return false
	}

	for _, o := range allowedOrigins {
		if origin == o {
			return true
		}
	}
	return false
}

func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowedOriginsOnce.Do(loadAllowedOriginsFromEnv)

		origin := normalizeAllowedOrigin(r.Header.Get("Origin"))

		if origin != "" && isOriginAllowed(origin) {
			// Echo the origin. Browsers reject "*" when credentials/cookies are used.
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		} else if origin == "" && isOriginAllowed("*") {
			// Non-browser clients.
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}

		// Legacy Python sets the string literal "true".
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		// w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Hub-Signature, X-Hub-Signature-256, X-GitHub-Event, X-GitHub-Delivery")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
