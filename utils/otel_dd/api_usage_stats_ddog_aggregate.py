#!/usr/bin/env python3

# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Query Datadog span events and output per-route API usage statistics as CSV.

This version is optimized for larger windows by using Datadog's aggregate API
for route counts and then (optionally) issuing targeted limit=1 searches per
route to fetch the first/last timestamps.
"""

from __future__ import annotations

import argparse
import csv
import datetime as dt
import json
import math
import os
import random
import re
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from typing import Any, Dict, List, Mapping, Optional, Sequence, Tuple


_ENV_RE = re.compile(r"(^|\s)env\s*:")
_SPECIAL_QUERY_CHARS = set('?><:="~/\\')
# Keep this sanitizer aligned with the backend OTel/DataDog route templating.
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
_RE_COMPANY_USER_CLA_MANAGER_DESIGNEE = re.compile(
    r"^(/v[0-9]+/company/[^/]+/user)/[^/]+(/claGroupID/[^/]+/is-cla-manager-designee)$"
)
_RE_CLA_MANAGER_USER = re.compile(r"^(/v[0-9]+/company/[^/]+/project/[^/]+/cla-manager)/[^/]+$")
_RE_REPOSITORY_PROVIDER_GITHUB_SIGN_NUMERIC = re.compile(
    r"^(/v[0-9]+/repository-provider/github/sign/[^/]+)/[0-9]+(/[^/]+)$"
)
_RE_SIGNED_INDIVIDUAL_GITHUB_NUMERIC = re.compile(
    r"^(/v[0-9]+/signed/individual/[^/]+)/[0-9]+(/[^/]+)$"
)


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
    prev = None
    while p != prev:
        prev = p
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



HELP_TEXT = """
Fast Datadog span API usage stats.

Behavior:
  - Aggregate mode (default):
      1) fetch route counts via Datadog aggregate API
      2) fetch first/last with 2 targeted limit=1 searches per route
  - Scan mode:
      fallback / compatibility mode that scans all matching spans client-side

Required env vars:
  DD_SITE
  DD_API_KEY
  DD_APP_KEY

Examples:
  %(prog)s --env prod --skip-e2e --from now-14d > api_usage.csv
  %(prog)s --env prod --skip-e2e --from now-14d --counts-only > api_usage_counts.csv
  %(prog)s --env prod --skip-e2e --from now-14d --route-prefix /v1 --route-prefix /v2 > api_usage_v1_v2.csv
  %(prog)s --env prod --skip-e2e --from now-14d --language python > api_usage_python.csv
  %(prog)s --env prod --skip-e2e --from now-14d --language go > api_usage_go.csv
  %(prog)s --env prod --skip-e2e --from now-14d --mode scan > api_usage_scan.csv
  %(prog)s --env prod --skip-e2e --from now-14d --sanitize-routes > api_usage_sanitized.csv

Notes:
  - Aggregate mode is much faster for long windows.
  - --counts-only is the fastest mode because it performs only the aggregate request.
  - Exact first/last still requires 2 targeted lookups per route.
  - If aggregate mode returns no routes in your Datadog org, try:
      --route-facet http.route --route-query-attr http.route
    or:
      --route-facet custom.http.route --route-query-attr custom.http.route
  - You may pass comma-separated values for --route-facet, --route-query-attr,
    and --language-query-attrs; the script will try them in order.
  - Additional client-side route sanitization is disabled by default; enable with --sanitize-routes.
"""


def has_env_filter(query: str) -> bool:
    return bool(_ENV_RE.search(query))


def eprint(*args: Any) -> None:
    print(*args, file=sys.stderr)


def parse_ts(ts: str) -> dt.datetime:
    ts = ts.strip()
    if ts.endswith("Z"):
        ts = ts[:-1] + "+00:00"
    return dt.datetime.fromisoformat(ts).astimezone(dt.timezone.utc)


def fmt_ts(ts: dt.datetime) -> str:
    ts_utc = ts.astimezone(dt.timezone.utc)
    return ts_utc.strftime("%Y-%m-%d %H:%M:%S.%f")[:-3]


def is_e2e_true(span: Dict[str, Any]) -> bool:
    attrs = span.get("attributes") or {}
    custom = attrs.get("custom") or {}
    easycla = custom.get("easycla") or {}
    v = easycla.get("e2e", False)
    return str(v).strip().lower() == "true"


def extract_route(span: Dict[str, Any], *, sanitize_routes: bool = False) -> Optional[str]:
    attrs = span.get("attributes") or {}
    custom = attrs.get("custom") or {}

    http = custom.get("http") or {}
    route = http.get("route")
    if isinstance(route, str) and route.strip():
        value = route.strip()
        return sanitize_api_path(value) if sanitize_routes else value

    resource_name = attrs.get("resource_name")
    if isinstance(resource_name, str):
        rn = resource_name.strip()
        parts = rn.split(None, 1)
        if len(parts) == 2 and parts[1].startswith("/"):
            value = parts[1].strip()
            return sanitize_api_path(value) if sanitize_routes else value
        if rn.startswith("/"):
            return sanitize_api_path(rn) if sanitize_routes else rn

    return None


def extract_event_time(span: Dict[str, Any]) -> Optional[dt.datetime]:
    attrs = span.get("attributes") or {}
    ts = attrs.get("start_timestamp") or attrs.get("end_timestamp")
    if not isinstance(ts, str) or not ts.strip():
        return None
    try:
        return parse_ts(ts)
    except Exception:
        return None


def datadog_post_json(
    url: str,
    headers: Dict[str, str],
    payload: Dict[str, Any],
    timeout_s: int = 30,
    retries: int = 5,
) -> Dict[str, Any]:
    body = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(url, data=body, headers=headers, method="POST")
    last_err: Optional[Exception] = None

    for attempt in range(retries):
        try:
            with urllib.request.urlopen(req, timeout=timeout_s) as resp:
                raw = resp.read()
                return json.loads(raw.decode("utf-8"))
        except urllib.error.HTTPError as e:
            raw = e.read().decode("utf-8", errors="replace")
            if e.code in (429, 500, 502, 503, 504) and attempt + 1 < retries:
                sleep_s = min(30.0, (2 ** attempt) + random.random())
                time.sleep(sleep_s)
                last_err = RuntimeError(f"Datadog HTTP {e.code}: {raw}")
                continue
            raise RuntimeError(f"Datadog HTTP {e.code}: {raw}") from e
        except urllib.error.URLError as e:
            if attempt + 1 < retries:
                sleep_s = min(30.0, (2 ** attempt) + random.random())
                time.sleep(sleep_s)
                last_err = RuntimeError(f"Datadog request failed: {e}")
                continue
            raise RuntimeError(f"Datadog request failed: {e}") from e

    if last_err is not None:
        raise last_err
    raise RuntimeError("Datadog request failed")


class DatadogClient:
    def __init__(self, dd_site: str, dd_api_key: str, dd_app_key: str) -> None:
        self.headers = {
            "Content-Type": "application/json",
            "Accept": "application/json",
            "DD-API-KEY": dd_api_key,
            "DD-APPLICATION-KEY": dd_app_key,
        }
        self.search_url = f"https://api.{dd_site}/api/v2/spans/events/search"
        self.aggregate_url = f"https://api.{dd_site}/api/v2/spans/analytics/aggregate"

    def search(
        self,
        *,
        query: str,
        time_from: str,
        time_to: str,
        limit: int,
        sort: str = "timestamp",
        cursor: Optional[str] = None,
        timeout_s: int = 30,
    ) -> Dict[str, Any]:
        payload: Dict[str, Any] = {
            "data": {
                "type": "search_request",
                "attributes": {
                    "filter": {
                        "from": time_from,
                        "to": time_to,
                        "query": query,
                    },
                    "sort": sort,
                    "page": {"limit": limit},
                },
            }
        }
        if cursor:
            payload["data"]["attributes"]["page"]["cursor"] = cursor
        return datadog_post_json(self.search_url, self.headers, payload, timeout_s=timeout_s)

    def aggregate(
        self,
        *,
        query: str,
        time_from: str,
        time_to: str,
        route_facet: str,
        route_limit: int,
        timeout_s: int = 30,
    ) -> Dict[str, Any]:
        payload: Dict[str, Any] = {
            "data": {
                "type": "aggregate_request",
                "attributes": {
                    "compute": [
                        {
                            "aggregation": "count",
                            "type": "total",
                        }
                    ],
                    "filter": {
                        "from": time_from,
                        "to": time_to,
                        "query": query,
                    },
                    "group_by": [
                        {
                            "facet": route_facet,
                            "limit": route_limit,
                            "sort": {
                                "type": "measure",
                                "aggregation": "count",
                                "order": "desc",
                            },
                        }
                    ],
                },
            }
        }
        return datadog_post_json(self.aggregate_url, self.headers, payload, timeout_s=timeout_s)


def _escape_attr_value(v: str) -> str:
    out: List[str] = []
    for ch in v:
        if ch in _SPECIAL_QUERY_CHARS or ch.isspace():
            out.append("\\" + ch)
        else:
            out.append(ch)
    return "".join(out)


def _attr_term(attr: str, value: str, wildcard: bool = False) -> str:
    escaped = _escape_attr_value(value)
    if wildcard:
        return f"@{attr}:{escaped}*"
    return f"@{attr}:{escaped}"


def parse_multi_attr_list(raw: Optional[str], defaults: Sequence[str]) -> List[str]:
    if not raw:
        return list(defaults)
    out: List[str] = []
    for piece in raw.split(","):
        value = piece.strip()
        if value:
            out.append(value)
    return out or list(defaults)


def build_query(
    base_query: str,
    *,
    env: str,
    ensure_env_filter: bool,
    skip_e2e: bool,
    pushdown_e2e: bool,
    e2e_attr: str,
    route_prefixes: Sequence[str],
    route_query_attrs: Sequence[str],
    languages: Sequence[str],
    language_query_attrs: Sequence[str],
) -> str:
    query = (base_query or "").strip()
    if env and ensure_env_filter and not has_env_filter(query):
        query = f"{query} env:{env}".strip() if query else f"env:{env}"

    parts: List[str] = [query] if query else []

    if skip_e2e and pushdown_e2e:
        parts.append(f"-@{e2e_attr}:true")

    if route_prefixes and route_query_attrs:
        prefix_blocks: List[str] = []
        for prefix in route_prefixes:
            prefix_terms = [_attr_term(attr, prefix, wildcard=True) for attr in route_query_attrs]
            if len(prefix_terms) == 1:
                prefix_blocks.append(prefix_terms[0])
            else:
                prefix_blocks.append("(" + " OR ".join(prefix_terms) + ")")
        parts.append("(" + " OR ".join(prefix_blocks) + ")")

    if languages and language_query_attrs:
        lang_blocks: List[str] = []
        for language in languages:
            lang_terms = [_attr_term(attr, language, wildcard=False) for attr in language_query_attrs]
            if len(lang_terms) == 1:
                lang_blocks.append(lang_terms[0])
            else:
                lang_blocks.append("(" + " OR ".join(lang_terms) + ")")
        parts.append("(" + " OR ".join(lang_blocks) + ")")

    return " ".join(part for part in parts if part).strip()


def extract_first_numeric(value: Any) -> Optional[float]:
    if isinstance(value, bool):
        return None
    if isinstance(value, (int, float)) and math.isfinite(float(value)):
        return float(value)
    if isinstance(value, Mapping):
        for key in ("value", "count", "result"):
            if key in value:
                out = extract_first_numeric(value[key])
                if out is not None:
                    return out
        for nested in value.values():
            out = extract_first_numeric(nested)
            if out is not None:
                return out
        return None
    if isinstance(value, (list, tuple)):
        for item in value:
            out = extract_first_numeric(item)
            if out is not None:
                return out
        return None
    return None


def aggregate_route_counts(
    client: DatadogClient,
    *,
    query: str,
    time_from: str,
    time_to: str,
    route_facets: Sequence[str],
    route_limit: int,
    verbose: bool,
    sanitize_routes: bool,
) -> Tuple[Dict[str, int], Dict[str, List[str]], Optional[str]]:
    errors: List[str] = []

    for route_facet in route_facets:
        if verbose:
            eprint(f"[ddog] aggregate counts by {route_facet!r} ...")
        try:
            resp = client.aggregate(
                query=query,
                time_from=time_from,
                time_to=time_to,
                route_facet=route_facet,
                route_limit=route_limit,
            )
        except Exception as exc:
            errors.append(f"{route_facet}: {exc}")
            continue

        meta = resp.get("meta") or {}
        if meta.get("status") == "timeout":
            warnings = meta.get("warnings") or []
            errors.append(f"{route_facet}: aggregate timeout warnings={warnings!r}")
            continue

        data = resp.get("data") or []
        if not isinstance(data, list):
            errors.append(f"{route_facet}: unexpected aggregate response shape")
            continue

        stats: Dict[str, int] = {}
        route_groups: Dict[str, List[str]] = {}
        for bucket in data:
            attrs = bucket.get("attributes") or {}
            by = attrs.get("by") or {}
            if not isinstance(by, dict):
                continue

            raw_route: Optional[str] = None
            route_candidates = [
                route_facet,
                route_facet[1:] if route_facet.startswith("@") else route_facet,
                "http.route",
                "custom.http.route",
                "route",
            ]
            for key in route_candidates:
                value = by.get(key)
                if isinstance(value, str) and value.strip():
                    raw_route = value.strip()
                    break

            if not raw_route:
                continue

            count_val = None
            for key in ("compute", "computes"):
                if key in attrs:
                    count_val = extract_first_numeric(attrs.get(key))
                    if count_val is not None:
                        break
            if count_val is None:
                continue

            output_route = sanitize_api_path(raw_route) if sanitize_routes else raw_route
            stats[output_route] = stats.get(output_route, 0) + int(round(count_val))
            route_groups.setdefault(output_route, []).append(raw_route)

        if stats:
            normalized_groups = {route: sorted(set(raw_routes)) for route, raw_routes in route_groups.items()}
            return stats, normalized_groups, route_facet

        errors.append(f"{route_facet}: no route buckets returned")

    raise RuntimeError("; ".join(errors) if errors else "aggregate query returned no data")


def search_first_last_for_route(
    client: DatadogClient,
    *,
    base_query: str,
    route: str,
    time_from: str,
    time_to: str,
    route_query_attrs: Sequence[str],
    sort: str,
) -> Optional[dt.datetime]:
    for attr in route_query_attrs:
        route_term = _attr_term(attr, route, wildcard=False)
        query = f"{base_query} {route_term}".strip()
        resp = client.search(
            query=query,
            time_from=time_from,
            time_to=time_to,
            limit=1,
            sort=sort,
        )
        data = resp.get("data") or []
        if isinstance(data, list) and data:
            ts = extract_event_time(data[0])
            if ts is not None:
                return ts
    return None


def fetch_route_boundaries(
    client: DatadogClient,
    *,
    routes: Sequence[str],
    base_query: str,
    time_from: str,
    time_to: str,
    route_query_attrs: Sequence[str],
    workers: int,
    verbose: bool,
) -> Dict[str, Tuple[Optional[dt.datetime], Optional[dt.datetime]]]:
    out: Dict[str, Tuple[Optional[dt.datetime], Optional[dt.datetime]]] = {}

    if workers <= 1:
        for route in routes:
            first = search_first_last_for_route(
                client,
                base_query=base_query,
                route=route,
                time_from=time_from,
                time_to=time_to,
                route_query_attrs=route_query_attrs,
                sort="timestamp",
            )
            last = search_first_last_for_route(
                client,
                base_query=base_query,
                route=route,
                time_from=time_from,
                time_to=time_to,
                route_query_attrs=route_query_attrs,
                sort="-timestamp",
            )
            out[route] = (first, last)
        return out

    def job(route: str, sort: str) -> Tuple[str, str, Optional[dt.datetime]]:
        ts = search_first_last_for_route(
            client,
            base_query=base_query,
            route=route,
            time_from=time_from,
            time_to=time_to,
            route_query_attrs=route_query_attrs,
            sort=sort,
        )
        return route, sort, ts

    total_jobs = len(routes) * 2
    done = 0
    partial: Dict[str, Dict[str, Optional[dt.datetime]]] = {}

    with ThreadPoolExecutor(max_workers=workers) as executor:
        futures = [executor.submit(job, route, sort) for route in routes for sort in ("timestamp", "-timestamp")]
        for future in as_completed(futures):
            route, sort, ts = future.result()
            partial.setdefault(route, {})[sort] = ts
            done += 1
            if verbose and (done == total_jobs or done % max(10, workers) == 0):
                eprint(f"[ddog] boundary lookups: {done}/{total_jobs}")

    for route in routes:
        item = partial.get(route, {})
        out[route] = (item.get("timestamp"), item.get("-timestamp"))

    return out

def collapse_route_boundaries(
    route_groups: Mapping[str, Sequence[str]],
    raw_boundaries: Mapping[str, Tuple[Optional[dt.datetime], Optional[dt.datetime]]],
) -> Dict[str, Tuple[Optional[dt.datetime], Optional[dt.datetime]]]:
    out: Dict[str, Tuple[Optional[dt.datetime], Optional[dt.datetime]]] = {}
    for route, raw_routes in route_groups.items():
        first_candidates: List[dt.datetime] = []
        last_candidates: List[dt.datetime] = []
        for raw_route in raw_routes:
            first, last = raw_boundaries.get(raw_route, (None, None))
            if first is not None:
                first_candidates.append(first)
            if last is not None:
                last_candidates.append(last)
        out[route] = (
            min(first_candidates) if first_candidates else None,
            max(last_candidates) if last_candidates else None,
        )
    return out


def fetch_spans(
    client: DatadogClient,
    *,
    query: str,
    time_from: str,
    time_to: str,
    limit: int,
    verbose: bool,
) -> List[Dict[str, Any]]:
    all_data: List[Dict[str, Any]] = []
    cursor: Optional[str] = None
    page_num = 0

    while True:
        page_num += 1
        if verbose:
            eprint(f"[ddog] fetching page {page_num} (cursor={cursor!r}) ...")

        resp = client.search(
            query=query,
            time_from=time_from,
            time_to=time_to,
            limit=limit,
            sort="timestamp",
            cursor=cursor,
        )
        data = resp.get("data") or []
        if not isinstance(data, list):
            raise RuntimeError("Unexpected Datadog response: 'data' is not a list")

        all_data.extend(data)

        meta = resp.get("meta") or {}
        page = meta.get("page") or {}
        next_cursor = None
        if isinstance(page, dict):
            next_cursor = page.get("after") or page.get("cursor") or page.get("next_cursor")

        if not next_cursor or len(data) == 0:
            break
        cursor = str(next_cursor)

    return all_data


def scan_mode_rows(
    client: DatadogClient,
    *,
    query: str,
    time_from: str,
    time_to: str,
    limit: int,
    verbose: bool,
    skip_e2e: bool,
    route_prefixes: Sequence[str],
    sanitize_routes: bool,
) -> Tuple[List[Tuple[str, int, dt.datetime, dt.datetime]], Dict[str, int]]:
    spans = fetch_spans(
        client,
        query=query,
        time_from=time_from,
        time_to=time_to,
        limit=limit,
        verbose=verbose,
    )

    stats: Dict[str, Tuple[int, dt.datetime, dt.datetime]] = {}
    kept = 0
    skipped_e2e = 0
    skipped_missing_route = 0
    skipped_missing_ts = 0

    for span in spans:
        if skip_e2e and is_e2e_true(span):
            skipped_e2e += 1
            continue

        raw_route = extract_route(span)
        if not raw_route:
            skipped_missing_route += 1
            continue

        if route_prefixes and not any(raw_route.startswith(prefix) for prefix in route_prefixes):
            continue
        route = sanitize_api_path(raw_route) if sanitize_routes else raw_route

        ts = extract_event_time(span)
        if not ts:
            skipped_missing_ts += 1
            continue

        kept += 1
        if route not in stats:
            stats[route] = (1, ts, ts)
        else:
            count, tmin, tmax = stats[route]
            stats[route] = (count + 1, min(tmin, ts), max(tmax, ts))

    rows = sorted(
        ((route, count, tmin, tmax) for route, (count, tmin, tmax) in stats.items()),
        key=lambda item: (-item[1], item[0]),
    )
    debug = {
        "spans_fetched": len(spans),
        "spans_kept": kept,
        "e2e_skipped": skipped_e2e,
        "no_route": skipped_missing_route,
        "no_ts": skipped_missing_ts,
        "routes": len(stats),
    }
    return rows, debug


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Datadog span API usage stats (CSV)",
        epilog=HELP_TEXT,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    group = parser.add_mutually_exclusive_group()
    group.add_argument("--skip-e2e", action="store_true", help="Skip e2e spans (default)")
    group.add_argument("--no-skip-e2e", action="store_true", help="Include e2e spans")

    parser.add_argument("--from", dest="time_from", default="now-60m", help='Time range start (Datadog format), default "now-60m"')
    parser.add_argument("--to", dest="time_to", default="now", help='Time range end (Datadog format), default "now"')
    parser.add_argument("--limit", type=int, default=5000, help="Page limit per search request in scan mode (default: 5000)")
    parser.add_argument("--verbose", action="store_true", help="Log progress to stderr")
    parser.add_argument("--sanitize-routes", action="store_true", help="Post-process extracted routes with the local sanitizer before grouping/output (default: off)")
    parser.add_argument(
        "--env",
        "--environment",
        "--stage",
        dest="env",
        default=os.getenv("DD_ENV") or os.getenv("ENV") or os.getenv("STAGE") or "dev",
        help='Datadog env tag value (default: DD_ENV/ENV/STAGE or "dev")',
    )
    parser.add_argument(
        "--query",
        default="service:easycla-backend",
        help='Datadog query string WITHOUT env (env is appended unless query already contains env:...) (default: "service:easycla-backend")',
    )

    parser.add_argument("--mode", choices=("aggregate", "scan"), default="aggregate", help="Use aggregate API for counts (default) or full scan mode")
    parser.add_argument("--counts-only", action="store_true", help="Output only api,n_calls using aggregate API")
    parser.add_argument(
        "--route-facet",
        default="custom.http.route,http.route",
        help="Comma-separated facet names to try for aggregate group_by (default: custom.http.route,http.route)",
    )
    parser.add_argument(
        "--route-query-attr",
        default="custom.http.route,http.route",
        help="Comma-separated attribute names to try for exact/prefix route filters (default: custom.http.route,http.route)",
    )
    parser.add_argument("--route-limit", type=int, default=1000, help="Maximum number of routes/buckets to request from aggregate API (default: 1000), if too low raise or use --mode scan instead")
    parser.add_argument(
        "--route-prefix",
        action="append",
        default=[],
        help="Restrict to routes starting with this prefix; may be repeated (example: --route-prefix /v1 --route-prefix /v2)",
    )
    parser.add_argument(
        "--language",
        action="append",
        default=[],
        help="Restrict to telemetry SDK language; may be repeated (example: --language python --language go)",
    )
    parser.add_argument(
        "--language-query-attrs",
        default="language,custom.telemetry.sdk.language",
        help="Comma-separated attribute names used for language filtering (default: language,custom.telemetry.sdk.language)",
    )
    parser.add_argument("--boundary-workers", type=int, default=8, help="Concurrent workers for first/last route lookups in aggregate mode (default: 8)")
    parser.add_argument(
        "--no-query-skip-e2e",
        action="store_true",
        help="Do not push e2e exclusion into the Datadog query in scan mode; ignored in aggregate mode",
    )
    parser.add_argument("--e2e-attr", default="custom.easycla.e2e", help="Attribute name used for server-side e2e exclusion (default: custom.easycla.e2e)")
    parser.add_argument("--no-fallback-scan", action="store_true", help="Do not fall back to full scan mode when aggregate mode fails")

    args = parser.parse_args()

    skip_e2e = not args.no_skip_e2e

    dd_site = os.getenv("DD_SITE")
    dd_api_key = os.getenv("DD_API_KEY")
    dd_app_key = os.getenv("DD_APP_KEY")
    missing = [
        name
        for name, value in (("DD_SITE", dd_site), ("DD_API_KEY", dd_api_key), ("DD_APP_KEY", dd_app_key))
        if not value
    ]
    if missing:
        eprint(f"ERROR: missing env var(s): {', '.join(missing)}")
        return 2

    route_facets = parse_multi_attr_list(args.route_facet, ["custom.http.route", "http.route"])
    route_query_attrs = parse_multi_attr_list(args.route_query_attr, ["custom.http.route", "http.route"])
    language_query_attrs = parse_multi_attr_list(
        args.language_query_attrs,
        ["language", "custom.telemetry.sdk.language"],
    )
    no_query_skip_e2e = args.no_query_skip_e2e if args.mode == "scan" else False

    query = build_query(
        args.query,
        env=args.env,
        ensure_env_filter=True,
        skip_e2e=skip_e2e,
        pushdown_e2e=not no_query_skip_e2e,
        e2e_attr=args.e2e_attr,
        route_prefixes=args.route_prefix,
        route_query_attrs=route_query_attrs,
        languages=args.language,
        language_query_attrs=language_query_attrs,
    )

    client = DatadogClient(dd_site=dd_site or "", dd_api_key=dd_api_key or "", dd_app_key=dd_app_key or "")

    if args.mode == "scan":
        rows, debug = scan_mode_rows(
            client,
            query=query,
            time_from=args.time_from,
            time_to=args.time_to,
            limit=args.limit,
            verbose=args.verbose,
            skip_e2e=skip_e2e,
            route_prefixes=args.route_prefix,
            sanitize_routes=args.sanitize_routes,
        )
        writer = csv.writer(sys.stdout, lineterminator="\n")
        writer.writerow(["api", "n_calls", "first", "last"])
        for route, count, first, last in rows:
            writer.writerow([route, count, fmt_ts(first), fmt_ts(last)])
        if args.verbose:
            eprint(f"[ddog] spans fetched: {debug['spans_fetched']}")
            eprint(f"[ddog] spans kept:   {debug['spans_kept']}")
            if skip_e2e:
                eprint(f"[ddog] e2e skipped:  {debug['e2e_skipped']}")
            eprint(f"[ddog] no-route:     {debug['no_route']}")
            eprint(f"[ddog] no-ts:        {debug['no_ts']}")
            eprint(f"[ddog] routes:       {debug['routes']}")
        return 0

    try:
        counts, route_groups, facet_used = aggregate_route_counts(
            client,
            query=query,
            time_from=args.time_from,
            time_to=args.time_to,
            route_facets=route_facets,
            route_limit=args.route_limit,
            verbose=args.verbose,
            sanitize_routes=args.sanitize_routes,
        )
        if args.verbose and facet_used:
            eprint(f"[ddog] aggregate facet used: {facet_used}")
    except Exception as exc:
        if args.no_fallback_scan:
            raise
        if args.verbose:
            eprint(f"[ddog] aggregate mode failed: {exc}")
            eprint("[ddog] falling back to scan mode ...")
        rows, debug = scan_mode_rows(
            client,
            query=query,
            time_from=args.time_from,
            time_to=args.time_to,
            limit=args.limit,
            verbose=args.verbose,
            skip_e2e=skip_e2e,
            route_prefixes=args.route_prefix,
            sanitize_routes=args.sanitize_routes,
        )
        writer = csv.writer(sys.stdout, lineterminator="\n")
        writer.writerow(["api", "n_calls", "first", "last"])
        for route, count, first, last in rows:
            writer.writerow([route, count, fmt_ts(first), fmt_ts(last)])
        if args.verbose:
            eprint(f"[ddog] spans fetched: {debug['spans_fetched']}")
            eprint(f"[ddog] spans kept:   {debug['spans_kept']}")
            eprint(f"[ddog] routes:       {debug['routes']}")
        return 0

    routes_sorted = sorted(counts.items(), key=lambda item: (-item[1], item[0]))
    writer = csv.writer(sys.stdout, lineterminator="\n")

    if args.counts_only:
        writer.writerow(["api", "n_calls"])
        for route, count in routes_sorted:
            writer.writerow([route, count])
        if args.verbose:
            eprint(f"[ddog] aggregate routes: {len(counts)}")
            if args.sanitize_routes:
                raw_route_count = len({raw_route for raw_routes in route_groups.values() for raw_route in raw_routes})
                eprint(f"[ddog] raw aggregate routes: {raw_route_count}")
        return 0

    boundary_source_routes = sorted({raw_route for raw_routes in route_groups.values() for raw_route in raw_routes})
    raw_boundaries = fetch_route_boundaries(
        client,
        routes=boundary_source_routes,
        base_query=query,
        time_from=args.time_from,
        time_to=args.time_to,
        route_query_attrs=route_query_attrs,
        workers=max(1, args.boundary_workers),
        verbose=args.verbose,
    )
    boundaries = collapse_route_boundaries(route_groups, raw_boundaries)

    writer.writerow(["api", "n_calls", "first", "last"])
    for route, count in routes_sorted:
        first, last = boundaries.get(route, (None, None))
        writer.writerow([
            route,
            count,
            fmt_ts(first) if first else "",
            fmt_ts(last) if last else "",
        ])

    if args.verbose:
        eprint(f"[ddog] aggregate routes: {len(counts)}")
        if args.sanitize_routes:
            eprint(f"[ddog] raw aggregate routes: {len(boundary_source_routes)}")
        eprint(f"[ddog] boundary queries: {len(boundary_source_routes) * 2}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
