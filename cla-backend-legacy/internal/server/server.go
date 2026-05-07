// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package server

import (
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/api"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/logging"
	"github.com/linuxfoundation/easycla/cla-backend-legacy/internal/telemetry"
)

const envOtelDatadogAPILogging = "OTEL_DATADOG_API_LOGGING"

var (
	legacyOTelInitOnce    sync.Once
	legacyOTelInitEnabled bool
)

func parseBoolish(v string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "y", "on":
		return true, true
	case "0", "false", "no", "n", "off":
		return false, true
	default:
		return false, false
	}
}

func legacyOTelEnabled() bool {
	raw := strings.TrimSpace(os.Getenv(envOtelDatadogAPILogging))
	if raw == "" {
		return false
	}

	enabled, ok := parseBoolish(raw)
	if !ok {
		logging.Warnf("LG:otel-datadog-disabled invalid_%s=%q", envOtelDatadogAPILogging, raw)
		return false
	}

	return enabled
}

func legacyStage() string {
	for _, key := range []string{"STAGE", "stage"} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return "dev"
}

func legacyVersion() string {
	for _, key := range []string{
		"DD_VERSION",
		"LEGACY_BUILD_COMMIT",
		"GIT_COMMIT",
		"AWS_LAMBDA_FUNCTION_VERSION",
	} {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return "1.0"
}

func initLegacyDatadogOTelBestEffort() bool {
	legacyOTelInitOnce.Do(func() {
		legacyOTelInitEnabled = false

		if !legacyOTelEnabled() {
			logging.Infof("LG:otel-datadog-disabled %s=%q", envOtelDatadogAPILogging, os.Getenv(envOtelDatadogAPILogging))
			return
		}

		defer func() {
			if r := recover(); r != nil {
				logging.Warnf("LG:otel-datadog-init-panic recovered=%v", r)
				legacyOTelInitEnabled = false
			}
		}()

		stage := legacyStage()
		version := legacyVersion()

		if err := telemetry.InitDatadogOTel(telemetry.DatadogOTelConfig{
			Stage:   stage,
			Service: "easycla-backend",
			Version: version,
		}); err != nil {
			logging.Infof("LG:otel-datadog-disabled init_err=%v", err)
			return
		}

		legacyOTelInitEnabled = true
		logging.Infof("LG:otel-datadog-enabled stage=%s service=easycla-backend version=%s", stage, version)
	})

	return legacyOTelInitEnabled
}

func wrapHTTPHandlerWithTelemetryBestEffort(next http.Handler) (wrapped http.Handler) {
	wrapped = next
	defer func() {
		if r := recover(); r != nil {
			logging.Warnf("LG:otel-datadog-wrap-panic recovered=%v", r)
			wrapped = next
		}
	}()
	return telemetry.WrapHTTPHandler(next)
}

func flushTelemetryAfterResponse(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		telemetry.ForceFlushBestEffort(250 * time.Millisecond)
	})
}

// NewHTTPHandler builds the HTTP handler for both Lambda (via adapter) and local runs.
//
// Note: router-level middleware already handles request logging and CORS.
func NewHTTPHandler() http.Handler {
	otelEnabled := initLegacyDatadogOTelBestEffort()
	h := api.NewHandlers()
	router := api.NewRouter(h)
	if !otelEnabled {
		return router
	}
	return flushTelemetryAfterResponse(wrapHTTPHandlerWithTelemetryBestEffort(router))
}
