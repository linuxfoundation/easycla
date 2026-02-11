// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// DatadogOTelConfig configures OTel SDK for exporting traces to the Datadog Lambda Extension.
type DatadogOTelConfig struct {
	Stage   string
	Service string
	Version string
}

var (
	ddInitOnce sync.Once
	ddInitErr  error
)

// InitDatadogOTel initializes the global OTel SDK (tracer provider + OTLP exporter).
// Safe to call multiple times (sync.Once). Never panics.
func InitDatadogOTel(cfg DatadogOTelConfig) error {
	ddInitOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Tags (prefer explicit DD_* env vars; fallback to stage/config).
		ddEnv := strings.TrimSpace(os.Getenv("DD_ENV"))
		if ddEnv == "" {
			ddEnv = stageToDDEnv(cfg.Stage)
		}

		ddService := strings.TrimSpace(os.Getenv("DD_SERVICE"))
		if ddService == "" {
			ddService = cfg.Service
		}

		ddVersion := strings.TrimSpace(os.Getenv("DD_VERSION"))
		if ddVersion == "" {
			ddVersion = cfg.Version
		}

		exporter, err := newOTLPHTTPExporter(ctx)
		if err != nil {
			ddInitErr = err
			return
		}

		// Vendor-neutral resource attributes (Datadog maps these automatically).
		res, err := resource.New(ctx,
			resource.WithFromEnv(),
			resource.WithTelemetrySDK(),
			resource.WithAttributes(
				attribute.String("service.name", ddService),
				attribute.String("service.version", ddVersion),
				attribute.String("deployment.environment.name", ddEnv),
			),
		)
		if err != nil {
			ddInitErr = err
			return
		}

		tp := sdktrace.NewTracerProvider(
			sdktrace.WithResource(res),
			// Batch exporter => async export (no per-request network IO).
			sdktrace.WithBatcher(exporter),
		)

		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(
			propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			),
		)
	})

	if ddInitErr != nil {
		log.Infof("LG:otel-datadog-init-failed err=%v", ddInitErr)
	}
	return ddInitErr
}

// WrapHTTPHandler instruments inbound HTTP requests using otelhttp and produces spans.
// Span name is "<METHOD> <PATH>" for easier usage monitoring by endpoint.
func WrapHTTPHandler(next http.Handler) http.Handler {
	// Precompile patterns once (WrapHTTPHandler is expected to be called once at init/cold-start).
	reMultiSlash := regexp.MustCompile(`/{2,}`)
	reSwagger := regexp.MustCompile(`^/v[0-9]+/swagger(?:\.[A-Za-z0-9]+)?$`)
	reAPIDocs := regexp.MustCompile(`^/v[0-9]+/api-docs$`)

	// Dynamic segment patterns (reduce cardinality).
	reUUID := regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	reNumeric := regexp.MustCompile(`^[0-9]+$`)
	reSFID := regexp.MustCompile(`(?i)^(?:00|a0)[A-Za-z0-9]{13,16}$`)
	reLFXID := regexp.MustCompile(`(?i)^lf[A-Za-z0-9]{16,22}$`)
	reHexLong := regexp.MustCompile(`(?i)^[0-9a-f]{16,}$`)

	looksLikeOpaqueID := func(seg string) bool {
		// Safety valve for long opaque IDs that aren't strictly UUID/hex/numeric.
		// Keep conservative to avoid masking normal route segments.
		if len(seg) < 24 {
			return false
		}
		hasLetter := false
		hasDigit := false
		for _, ch := range seg {
			switch {
			case 'a' <= ch && ch <= 'z', 'A' <= ch && ch <= 'Z':
				hasLetter = true
			case '0' <= ch && ch <= '9':
				hasDigit = true
			case ch == '-' || ch == '_':
				// ok
			default:
				return false
			}
		}
		return hasLetter && hasDigit
	}

	return otelhttp.NewHandler(
		next,
		"easycla-http",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			// LG: use this to have per distinct API URL datapoints
			// return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			// LG: use this to have per API endpoint datapoints (mask dynamic segments)
			/**/
			p := strings.TrimSpace(r.URL.Path)
			if p == "" {
				p = "/"
			}

			// Normalize slashes + trailing slash.
			p = reMultiSlash.ReplaceAllString(p, "/")
			if len(p) > 1 && strings.HasSuffix(p, "/") {
				p = strings.TrimRight(p, "/")
				if p == "" {
					p = "/"
				}
			}

			// Stable groupings for docs-like endpoints across versions.
			switch {
			case reSwagger.MatchString(p):
				p = "/v*/swagger"
			case reAPIDocs.MatchString(p):
				p = "/v*/api-docs"
			default:
				parts := strings.Split(p, "/")
				for i := 1; i < len(parts); i++ { // skip leading ""
					seg := parts[i]
					if seg == "" {
						continue
					}
					if strings.EqualFold(seg, "null") ||
						reNumeric.MatchString(seg) ||
						reUUID.MatchString(seg) ||
						reSFID.MatchString(seg) ||
						reLFXID.MatchString(seg) ||
						reHexLong.MatchString(seg) ||
						looksLikeOpaqueID(seg) {
						parts[i] = "*"
					}
				}
				p = strings.Join(parts, "/")
				if p == "" {
					p = "/"
				}
			}

			return fmt.Sprintf("%s %s", r.Method, p)
			/**/
		}),
	)
}

func newOTLPHTTPExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	// Standard overrides; default to Datadog Lambda Extension OTLP/HTTP.
	//
	// OTLP/HTTP env var rules:
	// - OTEL_EXPORTER_OTLP_TRACES_ENDPOINT is per-signal. If set, preserve its path verbatim
	//   (default to "/" if no path).
	// - OTEL_EXPORTER_OTLP_ENDPOINT is a base endpoint. If set (and per-signal is not),
	//   append "/v1/traces" (handling trailing slashes).
	tracesEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"))
	baseEndpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))

	var (
		endpoint           string
		usedTracesEndpoint bool
		usedBaseEndpoint   bool
	)

	if tracesEndpoint != "" {
		endpoint = tracesEndpoint
		usedTracesEndpoint = true
	} else if baseEndpoint != "" {
		endpoint = baseEndpoint
		usedBaseEndpoint = true
	} else {
		// Datadog Lambda Extension default (OTLP/HTTP).
		endpoint = "http://localhost:4318/v1/traces"
		// Default is already the full traces endpoint => treat like per-signal.
		usedTracesEndpoint = true
	}

	var host string
	parsedPath := ""
	insecure := true

	// Accept full URL or host:port[/path]
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, err
		}
		host = u.Host
		parsedPath = u.Path
		insecure = (u.Scheme == "http")
	} else {
		host = endpoint
		if strings.Contains(endpoint, "/") {
			parts := strings.SplitN(endpoint, "/", 2)
			host = parts[0]
			// Preserve remainder as path (empty remainder => "/")
			parsedPath = "/" + parts[1]
		}
	}

	// Normalize empty/missing paths to "/" (URL semantics)
	if strings.TrimSpace(parsedPath) == "" {
		parsedPath = "/"
	} else if !strings.HasPrefix(parsedPath, "/") {
		// Defensive (shouldn't happen with url.Parse)
		parsedPath = "/" + parsedPath
	}

	path := parsedPath
	if usedBaseEndpoint {
		// Base endpoint: append OTLP/HTTP traces path, handling trailing slashes.
		base := strings.TrimRight(parsedPath, "/")
		path = base + "/v1/traces"
	} else if usedTracesEndpoint {
		// Per-signal endpoint: preserve path verbatim (already normalized above)
		path = parsedPath
	}

	if strings.TrimSpace(host) == "" {
		return nil, fmt.Errorf("invalid OTLP endpoint: %q", endpoint)
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(host),
		otlptracehttp.WithURLPath(path),
		otlptracehttp.WithTimeout(2 * time.Second),
	}
	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	return otlptracehttp.New(ctx, opts...)
}

func stageToDDEnv(stage string) string {
	const prod = "prod"
	const staging = "staging"
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case prod:
		return prod
	case staging:
		return staging
	default:
		return "dev"
	}
}
