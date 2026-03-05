#!/usr/bin/env python3
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

"""
Query Datadog span events and output per-route API usage statistics as CSV.

Default behavior:
  - Skips spans marked as attributes.custom.easycla.e2e == "true"
  - Groups by templated route attributes.custom.http.route
  - Outputs: api,n_calls,first,last (sorted by n_calls desc)

Env vars required:
  DD_SITE       (e.g. datadoghq.com, datadoghq.eu, us3.datadoghq.com, ...)
  DD_API_KEY
  DD_APP_KEY

Example:
  ./utils/otel_dd/api_usage_stats_ddog.py --from now-60m --to now > api_usage.csv
  ./utils/otel_dd/api_usage_stats_ddog.py --no-skip-e2e | head
  ./utils/otel_dd/api_usage_stats_ddog.py --from now-24h --to now > api_usage.csv
  ./utils/otel_dd/api_usage_stats_ddog.py --verbose | head
"""

from __future__ import annotations

import argparse
import csv
import datetime as dt
import json
import os
import re
import sys
import urllib.error
import urllib.request
from typing import Any, Dict, List, Optional, Tuple


_ENV_RE = re.compile(r"(^|\s)env\s*:")

def has_env_filter(query: str) -> bool:
    return bool(_ENV_RE.search(query))

def eprint(*args: Any) -> None:
    print(*args, file=sys.stderr)


def parse_ts(ts: str) -> dt.datetime:
    # Datadog returns ISO 8601 with Z, e.g. "2026-02-26T08:25:15.686Z"
    # Convert to timezone-aware UTC datetime.
    ts = ts.strip()
    if ts.endswith("Z"):
        ts = ts[:-1] + "+00:00"
    return dt.datetime.fromisoformat(ts).astimezone(dt.timezone.utc)


def fmt_ts(ts: dt.datetime) -> str:
    # Format as "YYYY-MM-DD HH:MM:SS.mmm" (milliseconds)
    ts_utc = ts.astimezone(dt.timezone.utc)
    return ts_utc.strftime("%Y-%m-%d %H:%M:%S.%f")[:-3]


def is_e2e_true(span: Dict[str, Any]) -> bool:
    attrs = span.get("attributes") or {}
    custom = attrs.get("custom") or {}
    easycla = custom.get("easycla") or {}
    v = easycla.get("e2e", False)
    return str(v).strip().lower() == "true"


def extract_route(span: Dict[str, Any]) -> Optional[str]:
    """
    Prefer templated HTTP route:
      attributes.custom.http.route  -> "/v1/repository/{uuid}"

    Fallback:
      attributes.resource_name      -> "GET /v1/repository/{uuid}"  (strip method)
    """
    attrs = span.get("attributes") or {}
    custom = attrs.get("custom") or {}

    http = custom.get("http") or {}
    route = http.get("route")
    if isinstance(route, str) and route.strip():
        return route.strip()

    resource_name = attrs.get("resource_name")
    if isinstance(resource_name, str):
        rn = resource_name.strip()
        # Often "METHOD /path"
        parts = rn.split(None, 1)
        if len(parts) == 2 and parts[1].startswith("/"):
            return parts[1].strip()
        # Sometimes just "/path"
        if rn.startswith("/"):
            return rn

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


def datadog_post_json(url: str, headers: Dict[str, str], payload: Dict[str, Any], timeout_s: int = 30) -> Dict[str, Any]:
    body = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(url, data=body, headers=headers, method="POST")
    try:
        with urllib.request.urlopen(req, timeout=timeout_s) as resp:
            raw = resp.read()
            return json.loads(raw.decode("utf-8"))
    except urllib.error.HTTPError as e:
        raw = e.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"Datadog HTTP {e.code}: {raw}") from e
    except urllib.error.URLError as e:
        raise RuntimeError(f"Datadog request failed: {e}") from e


def fetch_spans(
    dd_site: str,
    dd_api_key: str,
    dd_app_key: str,
    query: str,
    time_from: str,
    time_to: str,
    limit: int,
    verbose: bool,
) -> List[Dict[str, Any]]:
    url = f"https://api.{dd_site}/api/v2/spans/events/search"
    headers = {
        "Content-Type": "application/json",
        "DD-API-KEY": dd_api_key,
        "DD-APPLICATION-KEY": dd_app_key,
    }

    payload: Dict[str, Any] = {
        "data": {
            "type": "search_request",
            "attributes": {
                "filter": {
                    "from": time_from,
                    "to": time_to,
                    "query": query,
                },
                "sort": "timestamp",
                "page": {"limit": limit},
            },
        }
    }

    all_data: List[Dict[str, Any]] = []
    cursor: Optional[str] = None
    page_num = 0

    while True:
        page_num += 1
        if cursor:
            payload["data"]["attributes"]["page"]["cursor"] = cursor
        else:
            payload["data"]["attributes"]["page"].pop("cursor", None)

        if verbose:
            eprint(f"[ddog] fetching page {page_num} (cursor={cursor!r}) ...")

        resp = datadog_post_json(url, headers, payload)
        data = resp.get("data") or []
        if not isinstance(data, list):
            raise RuntimeError("Unexpected Datadog response: 'data' is not a list")

        all_data.extend(data)

        meta = resp.get("meta") or {}
        page = meta.get("page") or {}

        # Datadog APIs commonly use meta.page.after as the next cursor.
        next_cursor = None
        if isinstance(page, dict):
            next_cursor = page.get("after") or page.get("cursor") or page.get("next_cursor")

        if not next_cursor:
            break
        if len(data) == 0:
            break  # safety
        cursor = str(next_cursor)

    return all_data


def main() -> int:
    p = argparse.ArgumentParser(description="Datadog span API usage stats (CSV)")
    # Match your bash ergonomics, but default to SKIP
    g = p.add_mutually_exclusive_group()
    g.add_argument("--skip-e2e", action="store_true", help="Skip e2e spans (default)")
    g.add_argument("--no-skip-e2e", action="store_true", help="Include e2e spans")

    p.add_argument("--from", dest="time_from", default="now-60m", help='Time range start (Datadog format), default "now-60m"')
    p.add_argument("--to", dest="time_to", default="now", help='Time range end (Datadog format), default "now"')
    p.add_argument("--limit", type=int, default=5000, help="Page limit per request (default: 5000)")
    p.add_argument("--verbose", action="store_true", help="Log progress to stderr")
    p.add_argument("--env", "--environment", "--stage", dest="env", default=os.getenv("DD_ENV") or os.getenv("ENV") or os.getenv("STAGE") or "dev", help='Datadog env tag value (default: DD_ENV/ENV/STAGE or "dev")')
    p.add_argument("--query", default="service:easycla-backend", help='Datadog query string WITHOUT env (env is appended unless query already contains env:...) (default: "service:easycla-backend")')

    args = p.parse_args()
    query = (args.query or "").strip()
    if args.env and not has_env_filter(query):
        query = f"{query} env:{args.env}".strip() if query else f"env:{args.env}"

    # Default skip-e2e unless explicitly --no-skip-e2e
    skip_e2e = True
    if args.no_skip_e2e:
        skip_e2e = False

    dd_site = os.getenv("DD_SITE")
    dd_api_key = os.getenv("DD_API_KEY")
    dd_app_key = os.getenv("DD_APP_KEY")

    missing = [k for k, v in (("DD_SITE", dd_site), ("DD_API_KEY", dd_api_key), ("DD_APP_KEY", dd_app_key)) if not v]
    if missing:
        eprint(f"ERROR: missing env var(s): {', '.join(missing)}")
        return 2

    spans = fetch_spans(
        dd_site=dd_site,  # type: ignore[arg-type]
        dd_api_key=dd_api_key,  # type: ignore[arg-type]
        dd_app_key=dd_app_key,  # type: ignore[arg-type]
        query=query,
        time_from=args.time_from,
        time_to=args.time_to,
        limit=args.limit,
        verbose=args.verbose,
    )

    # route -> (count, min_ts, max_ts)
    stats: Dict[str, Tuple[int, dt.datetime, dt.datetime]] = {}

    kept = 0
    skipped_e2e = 0
    skipped_missing_route = 0
    skipped_missing_ts = 0

    for span in spans:
        if skip_e2e and is_e2e_true(span):
            skipped_e2e += 1
            continue

        route = extract_route(span)
        if not route:
            skipped_missing_route += 1
            continue

        t = extract_event_time(span)
        if not t:
            skipped_missing_ts += 1
            continue

        kept += 1
        if route not in stats:
            stats[route] = (1, t, t)
        else:
            cnt, tmin, tmax = stats[route]
            stats[route] = (cnt + 1, min(tmin, t), max(tmax, t))

    # Sort by count desc, then route
    rows = sorted(((route, cnt, tmin, tmax) for route, (cnt, tmin, tmax) in stats.items()),
                  key=lambda x: (-x[1], x[0]))

    w = csv.writer(sys.stdout, lineterminator="\n")
    w.writerow(["api", "n_calls", "first", "last"])
    for route, cnt, tmin, tmax in rows:
        w.writerow([route, cnt, fmt_ts(tmin), fmt_ts(tmax)])

    if args.verbose:
        eprint(f"[ddog] spans fetched: {len(spans)}")
        eprint(f"[ddog] spans kept:   {kept}")
        if skip_e2e:
            eprint(f"[ddog] e2e skipped:  {skipped_e2e}")
        eprint(f"[ddog] no-route:     {skipped_missing_route}")
        eprint(f"[ddog] no-ts:        {skipped_missing_ts}")
        eprint(f"[ddog] routes:       {len(stats)}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
