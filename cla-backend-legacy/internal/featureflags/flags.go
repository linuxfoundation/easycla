package featureflags

import (
	"os"
	"strings"
	"sync"
)

var (
	cacheMu sync.Mutex
	cache   = map[string]bool{}
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

// EnabledByEnvOrStage mimics the legacy Python helper in cla/routes.py:
// - if ENV is set to a bool-ish value, that value wins
// - otherwise default to defaultNonProd if STAGE != prod, else defaultProd.
func EnabledByEnvOrStage(envVar string, defaultNonProd bool, defaultProd bool) bool {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if v, ok := cache[envVar]; ok {
		return v
	}

	raw := os.Getenv(envVar)
	if raw != "" {
		if parsed, ok := parseBoolish(raw); ok {
			cache[envVar] = parsed
			return parsed
		}
	}

	stage := strings.TrimSpace(strings.ToLower(os.Getenv("STAGE")))
	if stage == "" {
		stage = "dev"
	}
	isProd := stage == "prod"
	enabled := defaultNonProd
	if isProd {
		enabled = defaultProd
	}
	cache[envVar] = enabled
	return enabled
}
