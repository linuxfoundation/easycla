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

	log "github.com/linuxfoundation/easycla/cla-backend-legacy/internal/logging"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// DatadogOTelConfig configures OTel SDK for exporting traces to the Datadog Lambda Extension.
type DatadogOTelConfig struct {
	Stage   string
	Service string
	Version string
}

var (
	ddInitOnce          sync.Once
	ddInitErr           error
	ddExportSuccessOnce sync.Once
)

// ddLoggingExporter logs once when spans are successfully exported (i.e. accepted by OTLP endpoint).
// This is intentionally low-volume to avoid flooding logs in prod.
type ddLoggingExporter struct {
	inner sdktrace.SpanExporter
}

func (e ddLoggingExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	err := e.inner.ExportSpans(ctx, spans)
	if err == nil {
		ddExportSuccessOnce.Do(func() {
			log.Infof("LG:otel-datadog-export-success spans=%d", len(spans))
		})
	}
	return err
}

func (e ddLoggingExporter) Shutdown(ctx context.Context) error {
	return e.inner.Shutdown(ctx)
}

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

		// Force "service.version" (Datadog version) to be the build commit when available.
		// Env var DD_VERSION may exist, but commit should take precedence for consistency across Go+Python.
		ddVersion := strings.TrimSpace(cfg.Version)
		if ddVersion == "" {
			ddVersion = strings.TrimSpace(os.Getenv("DD_VERSION"))
		}
		if ddVersion == "" {
			ddVersion = "1.0"
		}

		exporter, err := newOTLPHTTPExporter(ctx)
		if err != nil {
			ddInitErr = err
			return
		}
		exporter = ddLoggingExporter{inner: exporter}

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
	// UUIDs: classify valid vs invalid (E2E often probes invalid IDs)
	reUUIDValid := regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reUUIDLike := regexp.MustCompile(`/[0-9A-Za-z]{8}-[0-9A-Za-z]{4}-[0-9A-Za-z]{4}-[0-9A-Za-z]{4}-[0-9A-Za-z]{12}(/|$)`)
	reUUIDHexDash36 := regexp.MustCompile(`/[0-9a-fA-F-]{36}(/|$)`)
	reNumericID := regexp.MustCompile(`/[0-9]+(/|$)`)
	reSFIDValid := regexp.MustCompile(`/(?:00|a0)[A-Za-z0-9]{13,16}(/|$)`)
	reSFIDLike := regexp.MustCompile(`/(?:00|a0)[^/]{1,32}(/|$)`)
	reLFXIDValid := regexp.MustCompile(`/lf[A-Za-z0-9]{16,22}(/|$)`)
	reLFXIDLike := regexp.MustCompile(`/lf[^/]{1,32}(/|$)`)
	reNull := regexp.MustCompile(`/null(/|$)`)
	reInvalidUUIDSeg := regexp.MustCompile(`/(?:invalid-uuid(?:-format)?|not-a-uuid)(/|$)`)
	reInvalidSFIDSeg := regexp.MustCompile(`/invalid-sfid(?:-format)?(/|$)`)

	boolishTrue := func(v string) bool {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "y", "on":
			return true
		default:
			return false
		}
	}

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
		// UUIDs: valid vs invalid
		p = reUUIDValid.ReplaceAllString(p, "{uuid}")
		p = reUUIDLike.ReplaceAllString(p, "/{invalid-uuid}$1")
		p = reUUIDHexDash36.ReplaceAllString(p, "/{invalid-uuid}$1")
		p = reNumericID.ReplaceAllString(p, "/{id}$1")
		// Salesforce IDs: valid vs invalid
		p = reSFIDValid.ReplaceAllString(p, "/{sfid}$1")
		p = reSFIDLike.ReplaceAllString(p, "/{invalid-sfid}$1")
		// LFX IDs: valid vs invalid
		p = reLFXIDValid.ReplaceAllString(p, "/{lfxid}$1")
		p = reLFXIDLike.ReplaceAllString(p, "/{invalid-lfxid}$1")
		p = reNull.ReplaceAllString(p, "/{null}$1")
		// Known "invalid" test tokens (Cypress) -> placeholders
		p = reInvalidUUIDSeg.ReplaceAllString(p, "/{invalid-uuid}$1")
		p = reInvalidSFIDSeg.ReplaceAllString(p, "/{invalid-sfid}$1")

		if p == "" {
			return "/"
		}
		return p
	}

	// We want:
	// - grouping by templated route => span name "METHOD /vN/thing/{uuid}" and attribute http.route="/vN/thing/{uuid}"
	// - raw/original path visible per span => url.path="/vN/thing/<uuid>" and http.target="/vN/thing/<uuid>?..."
	//
	// otelhttp doesn't know framework routes, so we set http.route ourselves after the span is started.
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawPath := "/"
		rawTarget := "/"

		if r != nil && r.URL != nil {
			// Prefer URL.Path if present
			if strings.TrimSpace(r.URL.Path) != "" {
				rawPath = r.URL.Path
			}

			// Default target to path; RequestURI includes query string when present.
			rawTarget = rawPath
			if strings.TrimSpace(r.URL.RequestURI()) != "" {
				rawTarget = r.URL.RequestURI()
			}
		}

		route := sanitize(rawPath)
		log.Debugf("Sanitized path: %q -> %q", rawPath, route)

		span := trace.SpanFromContext(r.Context())
		span.SetAttributes(
			attribute.String("http.route", route),
			attribute.String("url.path", rawPath),
			attribute.String("http.target", rawTarget),
		)

		// Optional E2E marker (lets us filter CI noise in Datadog).
		e2eVal := ""
		if r != nil {
			e2eVal = r.Header.Get("X-EasyCLA-E2E")
			if strings.TrimSpace(e2eVal) == "" {
				e2eVal = r.Header.Get("X-E2E-TEST")
			}
		}
		if boolishTrue(e2eVal) {
			runID := ""
			if r != nil {
				runID = strings.TrimSpace(r.Header.Get("X-EasyCLA-E2E-RunID"))
			}
			if runID != "" {
				span.SetAttributes(
					attribute.Bool("easycla.e2e", true),
					attribute.String("easycla.e2e_run_id", runID),
				)
			} else {
				span.SetAttributes(attribute.Bool("easycla.e2e", true))
				log.Debugf("Sanitized path: %q -> %q e2e=1", rawPath, route)
			}
		}

		next.ServeHTTP(w, r)
	})

	return otelhttp.NewHandler(
		inner,
		"easycla-http",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			path := "/"
			if r != nil && r.URL != nil && strings.TrimSpace(r.URL.Path) != "" {
				path = r.URL.Path
			}
			return fmt.Sprintf("%s %s", r.Method, sanitize(path))
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
