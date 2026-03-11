package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/logging"
)

const (
	e2eHeader       = "X-EasyCLA-E2E"
	e2eRunIDHeader  = "X-EasyCLA-E2E-RunID"
	e2eLegacyHeader = "X-E2E-TEST"
)

func parseBoolish(raw string) (bool, bool) {
	v := strings.TrimSpace(strings.ToLower(raw))
	switch v {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func extractE2EMarker(h http.Header) (bool, string) {
	if h == nil {
		return false, ""
	}
	raw := h.Get(e2eHeader)
	if strings.TrimSpace(raw) == "" {
		raw = h.Get(e2eLegacyHeader)
	}
	if ok, parsed := func() (bool, bool) {
		b, ok := parseBoolish(raw)
		return ok, b
	}(); ok && parsed {
		return true, strings.TrimSpace(h.Get(e2eRunIDHeader))
	}
	return false, ""
}

// RequestLog mirrors the legacy Python request middleware log lines:
// - LG:api-request-path:<path>
// - LG:e2e-request-path:<path> e2e=1 [e2e_run_id=...]
func RequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := "/"
		if r != nil && r.URL != nil && strings.TrimSpace(r.URL.Path) != "" {
			path = r.URL.Path
		}

		logging.Infof("LG:api-request-path:%s", path)

		if ok, runID := extractE2EMarker(r.Header); ok {
			suffix := " e2e=1"
			if runID != "" {
				suffix += fmt.Sprintf(" e2e_run_id=%s", runID)
			}
			logging.Infof("LG:e2e-request-path:%s%s", path, suffix)
		}

		next.ServeHTTP(w, r)
	})
}
