#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
# Legacy API (cla-backend-legacy, Go; it answers x-easycla-backend-version: go) on the direct host: backend header, v1 readers require auth (anonymous and
# api-gw user tokens both 401 - the legacy API only accepts its own Auth0 claim set, by design), v2
# last-signature carries no DocuSign secrets, anonymous public v2 readers still 200, allowlist request with
# markup refused by validation, and (prod) the lfcla.com alias only redirects.
set -u
. "$(cd "$(dirname "$0")/../lib" && pwd)/common.sh"
LB=$LEGACY_API

# ---- health + backend header --------------------------------------------------------------------------------
code=$(curl -s --max-time 30 -D "$E2E_EVIDENCE_DIR/health.headers" -o "$E2E_EVIDENCE_DIR/health.json" -w '%{http_code}' "$LB/v2/health")
hdr=$(grep -i '^x-easycla-backend' "$E2E_EVIDENCE_DIR/health.headers" | tr -d '\r' | head -1)
if [ "$code" = "200" ]; then e2e_result LG-HEALTH PASS "GET /v2/health -> 200 ${hdr:-(no x-easycla-backend header)}"
else e2e_result LG-HEALTH FAIL "GET /v2/health -> $code"; fi

# ---- V6: v1 readers need auth; api-gw tokens are not accepted either --------------------------------------
e2e_check LG-V1-SIGNATURE-ANON-401 anon 401 GET "$LB/v1/signature/$COMPANY_CCLA"
e2e_check LG-V1-SIGNATURES-COMPANY-ANON-401 anon 401 GET "$LB/v1/signatures/company/$COMPANY_ID"
e2e_check LG-V1-USER-SIGNATURES-ANON-401 anon 401 GET "$LB/v1/user/$ID1_UID/signatures"
# api-gw user token: prod legacy API refuses it (different Auth0 claim set, 401 by design), dev accepts it (200)
e2e_check LG-V1-SIGNATURE-GW-TOKEN "$ID1" "${LEGACY_V1_TOKEN_EXPECT:-401}" GET "$LB/v1/signature/$COMPANY_CCLA"
e2e_check LG-V1-USER-SIGNATURES-GW-TOKEN "$ID1" "${LEGACY_V1_TOKEN_EXPECT:-401}" GET "$LB/v1/user/$ID1_UID/signatures"
e2e_check LG-V1-USER-FROM-TOKEN-NO-ACCESS "$ID1" "401|404" GET "$LB/v1/user-from-token"

# ---- V6: v2 last-signature exposes no DocuSign secrets -----------------------------------------------------------
e2e_check LG-V2-LAST-SIGNATURE-ANON "anon" "200|404" GET "$LB/v2/user/$ID1_UID/project/$CLA_GROUP/last-signature"
leak=$(python3 -I - "$E2E_HTTP_OUT" <<'PY'
import sys
raw = open(sys.argv[1]).read()
print(",".join(k for k in ("user_docusign_raw_xml", "signature_envelope_id") if k in raw))
PY
)
urls=$(python3 -I - "$E2E_HTTP_OUT" <<'PY'
import sys
raw = open(sys.argv[1]).read()
print(",".join(k for k in ("signature_callback_url", "signature_sign_url", "signature_return_url") if k in raw))
PY
)
if [ -z "$leak" ]; then e2e_result LG-V2-LAST-SIGNATURE-NO-SECRETS PASS "last-signature body carries no DocuSign raw XML / envelope id"
else e2e_result LG-V2-LAST-SIGNATURE-NO-SECRETS FAIL "secret keys present in last-signature body: $leak"; fi
# ops#350 strips only the raw XML and the envelope id; the DocuSign callback/sign/return urls are still served (known, prod)
e2e_result LG-V2-LAST-SIGNATURE-URL-KEYS INFO "url keys still in the last-signature body: ${urls:-none}"

# ---- L: anonymous public v2 readers (unchanged) ------------------------------------------------------------------
e2e_check LG-V2-PROJECT-ANON-200 anon 200 GET "$LB/v2/project/$CLA_GROUP"
e2e_check LG-V2-PROJECT-COMPANIES-ANON-200 anon 200 GET "$LB/v2/project/$CLA_GROUP/companies"
e2e_check LG-V2-PROJECT-DOCUMENT-ANON anon "200|404" GET "$LB/v2/project/$CLA_GROUP/document/individual"
e2e_check LG-V2-USER-ANON-401 anon 401 GET "$LB/v2/user/$ID1_UID"
e2e_check LG-V2-ACTIVE-SIGNATURE-ANON anon "200|404" GET "$LB/v2/user/$ID1_UID/active-signature"

# ---- V7: allowlist request with markup / unknown e-mail -> validation refuses (legacy contract: HTTP 200 + errors body, no mail, no row)
stamp=$(date -u +%Y%m%d%H%M%S)
e2e_check LG-V2-ALLOWLIST-REQUEST-MARKUP anon "200|400|403|422" POST \
  "$LB/v2/user/$ID1_UID/request-company-whitelist/$COMPANY_ID" \
  "{\"user_name\":\"E2E <b>QA</b>\",\"user_email\":\"e2e-qa-nomatch-$stamp@example.invalid\",\"project_id\":\"$CLA_GROUP\",\"message\":\"<img src=x onerror=alert(1)> QA $stamp\",\"recipient_name\":\"x\",\"recipient_email\":\"x@example.invalid\"}"
if [ "$E2E_LAST_CODE" = "SKIP" ]; then :
elif [ "$E2E_LAST_CODE" = "200" ]; then
  if grep -q '"errors"' "$E2E_HTTP_OUT" 2>/dev/null && grep -q 'user_email' "$E2E_HTTP_OUT" 2>/dev/null; then
    e2e_result LG-V2-ALLOWLIST-REQUEST-REFUSED PASS "200 with errors.user_email (e-mail not on the caller's row -> no mail, no row)"
  else e2e_result LG-V2-ALLOWLIST-REQUEST-REFUSED FAIL "200 without the errors.user_email validation body: $(e2e_redact 140 < "$E2E_HTTP_OUT")"; fi
fi

# ---- alias host (prod: api.lfcla.com only 301-redirects) ---------------------------------------------------------------
if [ -n "${LEGACY_API_ALIAS:-}" ]; then
  code=$(curl -s --max-time 30 -o /dev/null -w '%{http_code}' "$LEGACY_API_ALIAS/v2/health")
  case "$code" in 301|302|308) e2e_result LG-ALIAS-REDIRECT PASS "GET $LEGACY_API_ALIAS/v2/health -> $code (redirect only)";;
    200) e2e_result LG-ALIAS-REDIRECT INFO "GET $LEGACY_API_ALIAS/v2/health -> 200 (alias serves directly)";;
    *) e2e_result LG-ALIAS-REDIRECT FAIL "GET $LEGACY_API_ALIAS/v2/health -> $code";; esac
else
  e2e_result LG-ALIAS-REDIRECT SKIP "LEGACY_API_ALIAS not configured for $E2E_ENV"
fi
e2e_summary_line
[ "$E2E_FAIL" = "0" ]
