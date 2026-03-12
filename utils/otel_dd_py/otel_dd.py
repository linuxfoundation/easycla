#!/usr/bin/env python3

# Example:
# ./cla-backend/.venv/bin/python ./utils/otel_dd_py/otel_dd.py 'https://example.com/v2/project/123'

import os
import re
import sys
from urllib.parse import urlparse

# --- Path sanitizer (same intent as your backend code; keeps /vN intact) ---
_RE_MULTI_SLASH = re.compile(r"/{2,}")
_RE_ASSET_EXT = re.compile(r"\.(png|svg|css|js|json|xml|htm|html)$")
_RE_SWAGGER_ASSET = re.compile(r"^(/v[0-9]+)/swagger\.\{asset\}$")
_RE_SWAGGER_JSON_RESOURCE = re.compile(r"^(/v[0-9]+/swagger\.json)/.+$")
_RE_SWAGGER_TEMPLATED_RESOURCE = re.compile(r"^(/v[0-9]+/swagger\.\{asset\})/.+$")
_RE_UUID_VALID = re.compile(r"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}")
_RE_UUID_LIKE = re.compile(r"/[0-9A-Za-z]{8}-[0-9A-Za-z]{4}-[0-9A-Za-z]{4}-[0-9A-Za-z]{4}-[0-9A-Za-z]{12}(/|$)")
_RE_UUID_HEXDASH_36 = re.compile(r"/[0-9a-fA-F-]{36}(/|$)")
_RE_NUMERIC_ID = re.compile(r"/[0-9]+(/|$)")
_RE_SFID_VALID = re.compile(r"/(?:00|a0)[A-Za-z0-9]{13,16}(/|$)")
_RE_SFID_LIKE = re.compile(r"/(?:00|a0)[^/]{1,32}(/|$)")
_RE_LFXID_VALID = re.compile(r"/lf[A-Za-z0-9]{16,22}(/|$)")
_RE_LFXID_LIKE = re.compile(r"/lf[^/]{1,32}(/|$)")
_RE_NULL = re.compile(r"/null(/|$)")
_RE_UNDEFINED = re.compile(r"/undefined(/|$)")
_RE_INVALID_UUID_SEG = re.compile(r"/(?:invalid-uuid(?:-format)?|not-a-uuid)(/|$)")
_RE_INVALID_SFID_SEG = re.compile(r"/invalid-sfid(?:-format)?(/|$)")
_RE_USERS_USERNAME = re.compile(r"^(/v[0-9]+/users/username)/[^/]+$")
_RE_COMPANY_NAME = re.compile(r"^(/v[0-9]+/company/name)/[^/]+$")
_RE_COMPANY_USER_CLA_MANAGER_DESIGNEE = re.compile(r"^(/v[0-9]+/company/[^/]+/user)/[^/]+(/claGroupID/[^/]+/is-cla-manager-designee)$")
_RE_CLA_MANAGER_USER = re.compile(r"^(/v[0-9]+/company/[^/]+/project/[^/]+/cla-manager)/[^/]+$")
_RE_REPOSITORY_PROVIDER_GITHUB_SIGN_NUMERIC = re.compile(r"^(/v[0-9]+/repository-provider/github/sign/[^/]+)/[0-9]+(/[^/]+)$")
_RE_SIGNED_INDIVIDUAL_GITHUB_NUMERIC = re.compile(r"^(/v[0-9]+/signed/individual/[^/]+)/[0-9]+(/[^/]+)$")

def sanitize_api_path(path: str) -> str:
    p = (path or "").strip()
    if not p:
        return "/"
    if not p.startswith("/"):
        p = "/" + p

    p = _RE_MULTI_SLASH.sub("/", p)
    if len(p) > 1 and p.endswith("/"):
        p = p[:-1]

    p = _RE_SWAGGER_JSON_RESOURCE.sub(r"\1/{resource}", p)
    p = _RE_SWAGGER_TEMPLATED_RESOURCE.sub(r"\1/{resource}", p)
    p = _RE_USERS_USERNAME.sub(r"\1/{name}", p)
    p = _RE_COMPANY_NAME.sub(r"\1/{name}", p)
    p = _RE_COMPANY_USER_CLA_MANAGER_DESIGNEE.sub(r"\1/{name}\2", p)
    p = _RE_CLA_MANAGER_USER.sub(r"\1/{name}", p)
    p = _RE_REPOSITORY_PROVIDER_GITHUB_SIGN_NUMERIC.sub(r"\1/{n}\2", p)
    p = _RE_SIGNED_INDIVIDUAL_GITHUB_NUMERIC.sub(r"\1/{n}\2", p)
    p = _RE_ASSET_EXT.sub(".{asset}", p)
    p = _RE_SWAGGER_ASSET.sub(r"\1/swagger", p)

    p = _RE_UUID_VALID.sub("{uuid}", p)
    p = _RE_UUID_LIKE.sub(r"/{invalid-uuid}\1", p)
    p = _RE_UUID_HEXDASH_36.sub(r"/{invalid-uuid}\1", p)
    p = _RE_NUMERIC_ID.sub(r"/{id}\1", p)
    p = _RE_SFID_VALID.sub(r"/{sfid}\1", p)
    p = _RE_SFID_LIKE.sub(r"/{invalid-sfid}\1", p)
    p = _RE_LFXID_VALID.sub(r"/{lfxid}\1", p)
    p = _RE_LFXID_LIKE.sub(r"/{invalid-lfxid}\1", p)
    p = _RE_NULL.sub(r"/{null}\1", p)
    p = _RE_UNDEFINED.sub(r"/{undefined}\1", p)
    p = _RE_INVALID_UUID_SEG.sub(r"/{invalid-uuid}\1", p)
    p = _RE_INVALID_SFID_SEG.sub(r"/{invalid-sfid}\1", p)
    return p or "/"

def stage_to_dd_env(stage: str) -> str:
    st = (stage or "dev").strip().lower()
    if st in ("prod", "production"):
        return "prod"
    if st == "staging":
        return "staging"
    return "dev"

def build_otlp_traces_endpoint() -> str:
    """
    Preference order:
      1) OTEL_EXPORTER_OTLP_TRACES_ENDPOINT (preserve path)
      2) OTEL_EXPORTER_OTLP_ENDPOINT (append /v1/traces)
      3) default http://localhost:4318/v1/traces
    Accepts full URL or host:port[/path].
    """
    traces_ep = (os.getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") or "").strip()
    base_ep = (os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT") or "").strip()

    used_base = False
    if traces_ep:
        raw = traces_ep
    elif base_ep:
        raw = base_ep
        used_base = True
    else:
        raw = "http://localhost:4318/v1/traces"

    scheme = "http"
    host = ""
    path = "/"

    if raw.startswith("http://") or raw.startswith("https://"):
        u = urlparse(raw)
        scheme = u.scheme or "http"
        host = u.netloc
        path = u.path or "/"
    else:
        # host:port[/path]
        if "/" in raw:
            host, rest = raw.split("/", 1)
            path = "/" + rest if rest else "/"
        else:
            host = raw
            path = "/"

    if not path.startswith("/"):
        path = "/" + path
    if used_base:
        path = path.rstrip("/") + "/v1/traces"

    if not host.strip():
        raise ValueError(f"invalid OTLP endpoint: {raw!r}")

    return f"{scheme}://{host}{path}"

def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print(f"usage: {argv[0]} <url-or-path> [METHOD]", file=sys.stderr)
        return 2

    raw = argv[1]
    method = (argv[2] if len(argv) >= 3 else "GET").strip().upper() or "GET"

    # Extract path from URL if needed
    if raw.startswith("http://") or raw.startswith("https://"):
        path = urlparse(raw).path or "/"
    else:
        path = raw

    route = sanitize_api_path(path)
    span_name = f"{method} {route}"

    dd_env = (os.getenv("DD_ENV") or "").strip() or stage_to_dd_env(os.getenv("STAGE", "dev"))
    dd_service = (os.getenv("DD_SERVICE") or "").strip() or "easycla-backend"
    dd_version = (os.getenv("DD_VERSION") or "").strip() or (os.getenv("VERSION") or "").strip() or "1.0"

    endpoint = build_otlp_traces_endpoint()

    # Lazy imports (so the script can still show a clean error if deps are missing)
    try:
        from opentelemetry import trace as otel_trace
        from opentelemetry.sdk.resources import Resource
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import SimpleSpanProcessor
        from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
    except Exception as e:
        print(f"otel deps missing: {e}", file=sys.stderr)
        return 3

    resource = Resource.create({
        "service.name": dd_service,
        "service.version": dd_version,
        "deployment.environment.name": dd_env,
    })

    provider = TracerProvider(resource=resource)
    exporter = OTLPSpanExporter(endpoint=endpoint, timeout=2.0)
    provider.add_span_processor(SimpleSpanProcessor(exporter))
    otel_trace.set_tracer_provider(provider)
    tracer = otel_trace.get_tracer("easycla-otlp-poc")

    # Emit one span
    try:
        with tracer.start_as_current_span(span_name) as span:
            span.set_attribute("http.method", method)
            span.set_attribute("http.route", route)
            span.set_attribute("http.url", raw)
    finally:
        # Ensure it gets pushed before exit
        try:
            provider.force_flush()
        except Exception:
            pass
        try:
            provider.shutdown()
        except Exception:
            pass

    print(f"sent span: {span_name} -> {endpoint}")
    return 0

if __name__ == "__main__":
    raise SystemExit(main(sys.argv))

