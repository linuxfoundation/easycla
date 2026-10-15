#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
# Build under test: v4/v3 health, legacy health + X-EasyCLA-Backend header, legacy alias redirect.
set -uo pipefail
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

e2e_http anon GET "$API_V4/v4/ops/health"; code=$E2E_LAST_CODE
if [ "$code" = 200 ]; then
  info=$(python3 -I -c 'import sys,json; d=json.load(open(sys.argv[1])); print(json.dumps({"githash": d.get("Githash"), "build": d.get("BuildTimeStamp"), "status": d.get("Status"), "version": d.get("Version")}))' "$E2E_HTTP_OUT" 2>/dev/null || echo '{}')
  e2e_result HEALTH-V4 PASS "v4 /ops/health 200 $(printf '%s' "$info" | head -c 200)" "$info"
else
  e2e_result HEALTH-V4 FAIL "v4 /ops/health -> $code"
fi
e2e_check HEALTH-V3 anon "200|401" GET "$API_V4/v3/ops/health"

hdr="$E2E_EVIDENCE_DIR/legacy-health.headers"
code=$(curl -s --max-time 30 -D "$hdr" -o "$E2E_EVIDENCE_DIR/legacy-health.json" -w '%{http_code}' "$LEGACY_API/v2/health")
backend=$(grep -i '^x-easycla-backend' "$hdr" | tr -d '\r' | sed 's/^[^:]*: *//' | sed -n '$p')
if [ "$code" = 200 ]; then e2e_result HEALTH-LEGACY PASS "legacy /v2/health 200 (X-EasyCLA-Backend: ${backend:-absent})" "{\"backend\":$(e2e_json_escape "${backend:-}")}"
else e2e_result HEALTH-LEGACY FAIL "legacy /v2/health -> $code"; fi

if [ -n "${LEGACY_API_ALIAS:-}" ]; then
  code=$(curl -s --max-time 30 -o /dev/null -w '%{http_code}' "$LEGACY_API_ALIAS/v2/health")
  case "$code" in
    200|301|302|308) e2e_result HEALTH-LEGACY-ALIAS PASS "$LEGACY_API_ALIAS/v2/health -> $code" ;;
    *) e2e_result HEALTH-LEGACY-ALIAS FAIL "$LEGACY_API_ALIAS/v2/health -> $code" ;;
  esac
fi
e2e_summary_line
[ "$E2E_FAIL" = 0 ]
