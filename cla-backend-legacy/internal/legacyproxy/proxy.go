package legacyproxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// EnvLegacyUpstreamBaseURL is the environment variable that enables the proxy.
//
// When set, all unported endpoints will be forwarded to this upstream (the existing
// legacy Python API).
const EnvLegacyUpstreamBaseURL = "LEGACY_UPSTREAM_BASE_URL"

// Proxy forwards HTTP requests to a configured upstream base URL.
//
// This is used to keep the new Go service 1:1 compatible while the Python implementation
// is ported incrementally ("strangler" pattern).
type Proxy struct {
	upstream *url.URL
	client   *http.Client
}

// NewFromEnv creates a Proxy from environment configuration.
//
// If EnvLegacyUpstreamBaseURL is empty, it returns (nil, nil) to signal that the proxy
// is disabled.
func NewFromEnv() (*Proxy, error) {
	base := strings.TrimSpace(os.Getenv(EnvLegacyUpstreamBaseURL))
	if base == "" {
		return nil, nil
	}
	return New(base)
}

func New(baseURL string) (*Proxy, error) {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", EnvLegacyUpstreamBaseURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("%s must include scheme and host, got %q", EnvLegacyUpstreamBaseURL, baseURL)
	}

	// Keep a conservative timeout below the Lambda timeout.
	// Provider timeout is 60s; leave some headroom.
	client := &http.Client{
		Timeout: 55 * time.Second,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DisableCompression:    true, // pass-through content-encoding from upstream
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	return &Proxy{upstream: u, client: client}, nil
}

// ServeHTTP proxies the request to the configured upstream.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p == nil || p.upstream == nil {
		http.Error(w, "legacy proxy not configured", http.StatusBadGateway)
		return
	}

	upstreamURL := p.rewriteURL(r.URL)

	// The incoming request body may be read by middleware; ensure we can forward.
	var body io.Reader
	if r.Body != nil {
		// Read the body fully so we can safely retry or log in the future.
		// These requests are typically small JSON payloads.
		b, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()
		body = bytes.NewReader(b)
		r.Body = io.NopCloser(bytes.NewReader(b)) // restore for potential downstream reads
	} else {
		body = http.NoBody
	}

	upReq, err := http.NewRequestWithContext(r.Context(), r.Method, upstreamURL.String(), body)
	if err != nil {
		http.Error(w, "failed to create upstream request", http.StatusBadGateway)
		return
	}

	copyHeaders(upReq.Header, r.Header)
	stripHopByHopHeaders(upReq.Header)
	// Ensure we don't accidentally send an invalid Host header through API Gateway / CloudFront.
	upReq.Host = p.upstream.Host

	// Preserve the original host for observability/debugging.
	if r.Host != "" {
		upReq.Header.Set("X-Forwarded-Host", r.Host)
	}
	if proto := firstHeader(r.Header, "X-Forwarded-Proto", "X-Forwarded-Protocol"); proto != "" {
		upReq.Header.Set("X-Forwarded-Proto", proto)
	}

	resp, err := p.client.Do(upReq)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, context.DeadlineExceeded) {
			status = http.StatusGatewayTimeout
		}
		http.Error(w, "upstream request failed", status)
		return
	}
	defer resp.Body.Close()

	// Copy headers (preserving multi-value headers like Set-Cookie).
	stripHopByHopHeaders(resp.Header)
	copyResponseHeaders(w.Header(), resp.Header)

	// Best-effort rewrite for redirects and cookies so the proxy domain behaves like a first-class API.
	incomingHost := stripPort(r.Host)
	upstreamHost := stripPort(p.upstream.Host)
	if incomingHost != "" && upstreamHost != "" {
		rewriteLocationHeader(w.Header(), upstreamHost, incomingHost)
		rewriteSetCookieDomains(w.Header(), upstreamHost, incomingHost)
	}

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

func (p *Proxy) rewriteURL(in *url.URL) *url.URL {
	// Join base path (if any) with request path.
	out := *p.upstream
	// Preserve query string.
	out.RawQuery = in.RawQuery
	// Preserve path.
	out.Path = singleJoiningSlash(p.upstream.Path, in.Path)
	out.RawPath = "" // keep it simple
	return &out
}

func singleJoiningSlash(a, b string) string {
	aslash := strings.HasSuffix(a, "/")
	bslash := strings.HasPrefix(b, "/")
	switch {
	case aslash && bslash:
		return a + b[1:]
	case !aslash && !bslash:
		if a == "" {
			return "/" + b
		}
		return a + "/" + b
	}
	return a + b
}

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func copyResponseHeaders(dst, src http.Header) {
	for k := range dst {
		// clear first to avoid duplicates when the writer already has defaults
		dst.Del(k)
	}
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// Hop-by-hop headers are defined in RFC 7230 section 6.1 and must not be forwarded.
func stripHopByHopHeaders(h http.Header) {
	// Remove headers listed in the Connection header.
	if c := h.Get("Connection"); c != "" {
		for _, f := range strings.Split(c, ",") {
			if f = strings.TrimSpace(f); f != "" {
				h.Del(f)
			}
		}
	}

	for _, k := range []string{
		"Connection",
		"Proxy-Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailer",
		"Transfer-Encoding",
		"Upgrade",
	} {
		h.Del(k)
	}
}

func firstHeader(h http.Header, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(h.Get(k)); v != "" {
			return v
		}
	}
	return ""
}

func stripPort(hostport string) string {
	// In API Gateway / Lambda, we typically won't have ports, but be safe.
	if i := strings.Index(hostport, ":"); i >= 0 {
		return hostport[:i]
	}
	return hostport
}

func rewriteLocationHeader(h http.Header, upstreamHost, incomingHost string) {
	loc := h.Get("Location")
	if loc == "" {
		return
	}
	u, err := url.Parse(loc)
	if err != nil {
		return
	}
	if u.Host == "" {
		return // relative redirect
	}
	if strings.EqualFold(stripPort(u.Host), upstreamHost) {
		u.Host = incomingHost
		h.Set("Location", u.String())
	}
}

func rewriteSetCookieDomains(h http.Header, upstreamHost, incomingHost string) {
	values := h.Values("Set-Cookie")
	if len(values) == 0 {
		return
	}
	newValues := make([]string, 0, len(values))
	for _, sc := range values {
		parts := strings.Split(sc, ";")
		outParts := make([]string, 0, len(parts))
		for _, p := range parts {
			pTrim := strings.TrimSpace(p)
			if strings.HasPrefix(strings.ToLower(pTrim), "domain=") {
				dom := strings.TrimSpace(pTrim[len("domain="):])
				dom = strings.TrimPrefix(dom, ".")
				if strings.EqualFold(dom, upstreamHost) {
					outParts = append(outParts, "Domain="+incomingHost)
					continue
				}
			}
			outParts = append(outParts, pTrim)
		}
		newValues = append(newValues, strings.Join(outParts, "; "))
	}
	// Replace header values.
	h.Del("Set-Cookie")
	for _, v := range newValues {
		h.Add("Set-Cookie", v)
	}
}
