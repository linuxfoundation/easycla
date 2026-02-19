package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var (
	reMultiSlash   = regexp.MustCompile(`/{2,}`)
	reAssetExt     = regexp.MustCompile(`\.(png|svg|css|js|json|xml|htm|html)$`)
	reSwaggerAsset = regexp.MustCompile(`^(/v[0-9]+)/swagger\.\{asset\}$`)
	reUUID         = regexp.MustCompile(`[0-9a-fA-F-]{36}`)
	reNumericID    = regexp.MustCompile(`/[0-9]+(/|$)`)
	reSFID         = regexp.MustCompile(`/(?:00|a0)[A-Za-z0-9]{13,16}(/|$)`)
	reLFXID        = regexp.MustCompile(`/lf[A-Za-z0-9]{16,22}(/|$)`)
	reNull         = regexp.MustCompile(`/null(/|$)`)
)

func sanitizeAPIPath(path string) string {
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

	p = reAssetExt.ReplaceAllString(p, ".{asset}")
	if m := reSwaggerAsset.FindStringSubmatch(p); m != nil {
		p = m[1] + "/swagger"
	}

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

func stageToDDEnv(stage string) string {
	st := strings.ToLower(strings.TrimSpace(stage))
	switch st {
	case "prod", "production":
		return "prod"
	case "staging":
		return "staging"
	default:
		return "dev"
	}
}

// buildOTLPTracesEndpoint matches the same rules as your Go backend:
//  1. OTEL_EXPORTER_OTLP_TRACES_ENDPOINT (preserve path)
//  2. OTEL_EXPORTER_OTLP_ENDPOINT (append /v1/traces)
//  3. default http://localhost:4318/v1/traces
func buildOTLPTracesEndpoint() (host string, path string, insecure bool, err error) {
	traces := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"))
	base := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))

	raw := ""
	usedBase := false
	if traces != "" {
		raw = traces
	} else if base != "" {
		raw = base
		usedBase = true
	} else {
		raw = "http://localhost:4318/v1/traces"
	}

	insecure = true
	parsedPath := "/"

	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, e := url.Parse(raw)
		if e != nil {
			return "", "", false, e
		}
		host = u.Host
		parsedPath = u.Path
		insecure = (u.Scheme == "http")
	} else {
		// host:port[/path]
		host = raw
		if strings.Contains(raw, "/") {
			parts := strings.SplitN(raw, "/", 2)
			host = parts[0]
			if parts[1] != "" {
				parsedPath = "/" + parts[1]
			} else {
				parsedPath = "/"
			}
		}
	}

	if strings.TrimSpace(parsedPath) == "" {
		parsedPath = "/"
	}
	if !strings.HasPrefix(parsedPath, "/") {
		parsedPath = "/" + parsedPath
	}

	if usedBase {
		parsedPath = strings.TrimRight(parsedPath, "/") + "/v1/traces"
	}

	if strings.TrimSpace(host) == "" {
		return "", "", false, fmt.Errorf("invalid OTLP endpoint: %q", raw)
	}

	return host, parsedPath, insecure, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <url-or-path> [METHOD]\n", os.Args[0])
		os.Exit(2)
	}

	raw := os.Args[1]
	method := "GET"
	if len(os.Args) >= 3 {
		method = strings.ToUpper(strings.TrimSpace(os.Args[2]))
		if method == "" {
			method = "GET"
		}
	}

	pathOnly := raw
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		if u, err := url.Parse(raw); err == nil && u.Path != "" {
			pathOnly = u.Path
		} else {
			pathOnly = "/"
		}
	}

	route := sanitizeAPIPath(pathOnly)
	spanName := fmt.Sprintf("%s %s", method, route)

	ddEnv := strings.TrimSpace(os.Getenv("DD_ENV"))
	if ddEnv == "" {
		ddEnv = stageToDDEnv(os.Getenv("STAGE"))
	}
	ddService := strings.TrimSpace(os.Getenv("DD_SERVICE"))
	if ddService == "" {
		ddService = "easycla-backend"
	}
	ddVersion := strings.TrimSpace(os.Getenv("DD_VERSION"))
	if ddVersion == "" {
		ddVersion = strings.TrimSpace(os.Getenv("VERSION"))
	}
	if ddVersion == "" {
		ddVersion = "unknown"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	host, urlPath, insecure, err := buildOTLPTracesEndpoint()
	if err != nil {
		fmt.Fprintf(os.Stderr, "endpoint error: %v\n", err)
		os.Exit(3)
	}

	opts := []otlptracehttp.Option{
		otlptracehttp.WithEndpoint(host),
		otlptracehttp.WithURLPath(urlPath),
		otlptracehttp.WithTimeout(2 * time.Second),
	}
	if insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exp, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "exporter init error: %v\n", err)
		os.Exit(4)
	}

	res, _ := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", ddService),
			attribute.String("service.version", ddVersion),
			attribute.String("deployment.environment.name", ddEnv),
		),
	)

	// Sync export so the span is sent before process exit.
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSyncer(exp),
	)
	defer func() {
		_ = tp.Shutdown(ctx)
	}()

	otel.SetTracerProvider(tp)

	tr := otel.Tracer("easycla-otlp-poc")
	_, span := tr.Start(ctx, spanName)
	span.SetAttributes(
		attribute.String("http.method", method),
		attribute.String("http.route", route),
		attribute.String("http.url", raw),
	)
	span.End()

	_ = tp.ForceFlush(ctx)

	fmt.Printf("sent span: %s -> %s%s\n", spanName, host, urlPath)
}
