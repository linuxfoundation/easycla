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
func WrapHTTPHandler(next http.Handler) http.Handler {
	// Regexes mirror ./utils/count_apis.sh so OTel span names group the same way as the offline API log rollups:
	// - collapse multiple slashes
	// - trim trailing slash
	// - mask common asset extensions -> ".{asset}"
	// - normalize Swagger assets "/vN/swagger.{asset}" -> "/vN/swagger" (keep version; do NOT map to /v*)
	// - mask UUIDs, numeric IDs, Salesforce IDs, LFX IDs, and literal "null" segments
	reMultiSlash := regexp.MustCompile(`/{2,}`)
	reAssetExt := regexp.MustCompile(`\.(png|svg|css|js|json|xml|htm|html)$`)
	reSwaggerAsset := regexp.MustCompile(`^(/v[0-9]+)/swagger\.\{asset\}$`)
	reUUID := regexp.MustCompile(`[0-9a-fA-F-]{36}`)
	reNumericID := regexp.MustCompile(`/[0-9]+(/|$)`)
	reSFID := regexp.MustCompile(`/(?:00|a0)[A-Za-z0-9]{13,16}(/|$)`)
	reLFXID := regexp.MustCompile(`/lf[A-Za-z0-9]{16,22}(/|$)`)
	reNull := regexp.MustCompile(`/null(/|$)`)

	sanitize := func(path string) string {
		p := strings.TrimSpace(path)
		if p == "" {
			return "/"
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}

		p = reMultiSlash.ReplaceAllString(p, "/")
		if len(p) > 1 && strings.HasSuffix(p, "/") {
			p = strings.TrimSuffix(p, "/")
		}

		// Asset extensions (including swagger.json/xml/html) -> ".{asset}"
		p = reAssetExt.ReplaceAllString(p, ".{asset}")

		// Keep the version (/v1, /v2, ...) but normalize swagger asset paths.
		if m := reSwaggerAsset.FindStringSubmatch(p); m != nil {
			p = m[1] + "/swagger"
		}

		// Dynamic segment masking (use template placeholders, not "*")
		p = reUUID.ReplaceAllString(p, "{uuid}")
		p = reNumericID.ReplaceAllString(p, "/{id}$1")
		p = reSFID.ReplaceAllString(p, "/{sfid}$1")
		p = reLFXID.ReplaceAllString(p, "/{lfxid}$1")
		p = reNull.ReplaceAllString(p, "/{null}$1")

		if p == "" {
			return "/"
		}
		return p
	}

	return otelhttp.NewHandler(
		next,
		"easycla-http",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			// LG: use this to have per distinct API URL datapoints
			// return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
			return fmt.Sprintf("%s %s", r.Method, sanitize(r.URL.Path))
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
