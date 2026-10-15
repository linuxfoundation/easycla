#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
# Hardened route POST /v4/user/{userID}/company (ops#343 / acs-cli#1148 / lfx-gateway#116) and its neighbours.
# Every case except the opt-in positive one must NOT create an organization; identities: ID1 (own user), ID2
# (foreign user), anonymous.
set -uo pipefail
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../lib/common.sh"

STAMP=$E2E_TAG
payload() { # payload <name-suffix> <email>
  printf '{"companyName":"E2E QA %s %s","companyWebsite":"https://e2e-qa-%s-%s.invalid","userEmail":"%s"}' "$1" "$STAMP" "$(printf '%s' "$1" | tr 'A-Z ' 'a-z-')" "$STAMP" "$2"
}
U="$API_V4/v4/user"

# anonymous
e2e_check H-ANON-COMPANY anon 401 POST "$U/$ID1_UID/company" '{}'
e2e_check H-ANON-INVITE anon "401|422" POST "$U/$ID1_UID/invite-company-admin" '{}'
# ID1 targeting other users
e2e_check H-ID1-FOREIGN-UUID "$ID1" 403 POST "$U/00000000-0000-4000-8000-000000000000/company" "$(payload Foreign "$ID1_EMAIL")"
if [ -n "${OTHER_USER_UID:-}" ]; then
  e2e_check H-ID1-OTHER-REAL-USER "$ID1" 403 POST "$U/$OTHER_USER_UID/company" "$(payload Other "$ID1_EMAIL")"
else
  e2e_result H-ID1-OTHER-REAL-USER SKIP "OTHER_USER_UID not configured for $E2E_ENV"
fi
e2e_check H-ID1-PLATFORM-USERNAME "$ID1" 403 POST "$U/$ID1/company" "$(payload Platform "$ID1_EMAIL")"
e2e_check H-ID1-PLATFORM-SUB "$ID1" "403|422" POST "$U/auth0%7C$ID1/company" "$(payload Sub "$ID1_EMAIL")"
# ID1 own user: validation and duplicate rules without creating anything
e2e_check H-ID1-OWN-EMPTY-BODY "$ID1" 422 POST "$U/$ID1_UID/company" '{}'
# duplicate-website case: creation-capable if the website were unknown, so it is gated like the creating case
if [ -z "${COMPANY_WEBSITE:-}" ]; then
  e2e_result H-ID1-OWN-EXISTING-ORG SKIP "COMPANY_WEBSITE not configured for $E2E_ENV (duplicate-website case needs a real website)"
elif e2e_gate ALLOW_COMPANY_CREATE H-ID1-OWN-EXISTING-ORG "creation-capable request is disabled"; then
  e2e_check H-ID1-OWN-EXISTING-ORG "$ID1" "409|400|422" POST "$U/$ID1_UID/company" \
    "$(printf '{"companyName":%s,"companyWebsite":%s,"userEmail":%s}' "$(e2e_json_escape "$COMPANY_NAME")" "$(e2e_json_escape "$COMPANY_WEBSITE")" "$(e2e_json_escape "$ID1_EMAIL")")"
fi
# the only case that creates an organization - opt-in (dev only)
if e2e_gate ALLOW_COMPANY_CREATE H-ID1-OWN-OTHER-EMAIL-CREATE "positive company creation is disabled"; then
  e2e_check H-ID1-OWN-OTHER-EMAIL-CREATE "$ID1" 200 POST "$U/$ID1_UID/company" "$(payload Create "someone-else-$STAMP@example.invalid")"
  python3 -I -c 'import sys,json; d=json.load(open(sys.argv[1])); print("created company:", d.get("companyID") or d.get("companyId") or d.get("id"), d.get("companyName"))' "$E2E_HTTP_OUT" 2>/dev/null | tee -a "$E2E_LOG" || true
fi
# ID2 targeting ID1 (foreign) - must be refused
e2e_check H-ID2-FOREIGN-UID "$ID2" 403 POST "$U/$ID1_UID/company" "$(payload Neg "$ID2_EMAIL")"
e2e_check H-ID2-FOREIGN-USERNAME "$ID2" 403 POST "$U/$ID1/company" "$(payload NegU "$ID2_EMAIL")"
e2e_check H-ID2-FOREIGN-USERNAME-EMPTY "$ID2" "403|422" POST "$U/$ID1/company" '{}'
e2e_check H-ID2-INVITE-ADMIN-FOREIGN "$ID2" "403|422" POST "$U/$ID1_UID/invite-company-admin" '{}'
# user-from-token with an api-gw token: 401 (claim/audience mismatch, known and accepted)
e2e_check H-USER-FROM-TOKEN "$ID1" "401|200" GET "$API_V4/v4/user-from-token"

e2e_summary_line
[ "$E2E_FAIL" = 0 ]
