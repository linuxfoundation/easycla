package middleware

import (
	"encoding/json"
	"net/http"
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

func loadAllowedOriginsFromEnv() {
	raw := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS"))
	if raw == "" {
		// Backwards compatible default: allow all.
		allowAllOrigins = true
		return
	}
	// Supported formats:
	//  - JSON array: ["https://a", "https://b"]
	//  - CSV: https://a,https://b
	//  - Space/newline separated
	if strings.HasPrefix(raw, "[") {
		var arr []string
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			for _, v := range arr {
				v = strings.TrimSpace(strings.Trim(v, "\"'"))
				if v == "" {
					continue
				}
				allowedOrigins = append(allowedOrigins, v)
				if v == "*" {
					allowAllOrigins = true
				}
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
		p = strings.TrimSpace(strings.Trim(p, "\"'"))
		if p == "" {
			continue
		}
		allowedOrigins = append(allowedOrigins, p)
		if p == "*" {
			allowAllOrigins = true
		}
	}
	if len(allowedOrigins) == 0 {
		allowAllOrigins = true
	}
}

func isOriginAllowed(origin string) bool {
	allowedOriginsOnce.Do(loadAllowedOriginsFromEnv)
	if allowAllOrigins {
		return true
	}
	origin = strings.TrimSpace(origin)
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
		origin := r.Header.Get("Origin")
		if origin != "" && isOriginAllowed(origin) {
			// Echo the origin when allowlisting is enabled.
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
		} else if origin == "" && isOriginAllowed("*") {
			// Non-browser clients.
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && isOriginAllowed("*") {
			// Backwards compatible default: allow all.
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		// Legacy Python sets the string literal "true".
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		// Keep this list *exactly* aligned with the legacy Python middleware:
		//   response.set_header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
