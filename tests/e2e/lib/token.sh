#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
#
# Mint (or reuse) api-gw access tokens for the suite identities without modifying utils/get_lfx_token.sh:
# the script and its consent helper are staged in a private temp dir together with the identity's credentials
# file renamed to the name they expect (auth0-<env>.secret), the client-id file and the identity's refresh
# token, then the token is moved to
#   secrets/auth0-<env>-<identity>.token.secret
# and the refresh token to
#   secrets/lfx-<env>-<identity>-refresh.token.secret
# A cached token is reused while its JWT `exp` is more than 10 minutes away.
#
# usage: E2E_ENV=dev lib/token.sh [identity ...]      (default: ID1 and ID2 from config)
#        FORCE=1 to re-mint even when the cached token is still valid.
set -euo pipefail
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/common.sh"

token_seconds_left() { # prints seconds until exp (negative when expired / unparseable)
  python3 - "$1" <<'PY'
import base64, json, sys, time
try:
    payload = sys.argv[1].split(".")[1]
    payload += "=" * (-len(payload) % 4)
    exp = json.loads(base64.urlsafe_b64decode(payload)).get("exp", 0)
    print(int(exp - time.time()))
except Exception:
    print(-1)
PY
}

ensure_playwright() { # the headless device consent needs Playwright before run.sh installs it for the browser group
  [ -d "$E2E_ROOT/node_modules/playwright" ] && return 0
  e2e_log "installing playwright (npm install in tests/e2e) for the device consent"
  (cd "$E2E_ROOT" && npm install --no-audit --no-fund >/dev/null && npx --no-install playwright install chromium >/dev/null 2>&1)
}

mint_token() {
  local identity=$1 cached left work status=0 mode=login refresh limit=()
  cached="$(e2e_token "$identity")"
  if [ -n "$cached" ] && [ "${FORCE:-0}" != "1" ]; then
    left="$(token_seconds_left "$cached")"
    if [ "$left" -gt 600 ]; then e2e_log "token for $identity still valid for ${left}s - reusing"; return 0; fi
  fi
  e2e_require_secret "auth0-$E2E_ENV-$identity"
  refresh="$(e2e_secret_path "lfx-$E2E_ENV-$identity-refresh.token")"
  # get_lfx_token.sh resolves its files next to itself and the consent helper resolves Playwright via ../tests/e2e
  work="$(mktemp -d "${TMPDIR:-/tmp}/e2e-token.XXXXXX")"; chmod 700 "$work"
  mkdir -p "$work/utils" "$work/tests"; ln -s "$E2E_ROOT" "$work/tests/e2e"
  cp -p "$E2E_REPO_ROOT/utils/get_lfx_token.sh" "$E2E_REPO_ROOT/utils/lfx_device_consent.mjs" "$work/utils/"
  cp "$(e2e_secret_path "auth0-$E2E_ENV-$identity")" "$work/utils/auth0-$E2E_ENV.secret"
  local cid; cid="$(e2e_secret_path "lfx-$E2E_ENV-client-id")"
  [ -f "$cid" ] || cid="$E2E_REPO_ROOT/utils/lfx-$E2E_ENV-client-id.secret"
  if [ -f "$cid" ]; then cp "$cid" "$work/utils/lfx-$E2E_ENV-client-id.secret"; fi
  if [ -f "$refresh" ]; then cp "$refresh" "$work/utils/lfx-$E2E_ENV-refresh.token.secret"; mode=refresh; fi
  chmod 600 "$work/utils/"*.secret
  if command -v timeout >/dev/null 2>&1; then limit=(timeout 300); fi
  run_helper() { (cd "$work" && ${limit[@]+"${limit[@]}"} ./utils/get_lfx_token.sh "$E2E_ENV" "$1" >/dev/null 2>"$work/mint.err"); }
  e2e_log "minting $E2E_ENV token for $identity"
  if [ "$mode" = refresh ] && ! run_helper refresh; then mode=login; fi
  if [ "$mode" = login ] && ! { ensure_playwright && run_helper login; }; then
    status=1; e2e_log "token minting FAILED for $identity: $(grep -v -i -E 'https?://|user code' "$work/mint.err" 2>/dev/null | sed -n '$p' | e2e_redact 300)"
  else
    cp "$work/utils/auth0-$E2E_ENV.token.secret" "$(e2e_secret_path "auth0-$E2E_ENV-$identity.token")"
    if [ -f "$work/utils/lfx-$E2E_ENV-refresh.token.secret" ]; then cp "$work/utils/lfx-$E2E_ENV-refresh.token.secret" "$refresh"; chmod 600 "$refresh"; fi
    chmod 600 "$(e2e_secret_path "auth0-$E2E_ENV-$identity.token")"
    e2e_log "token for $identity saved (valid for $(token_seconds_left "$(e2e_token "$identity")")s)"
  fi
  rm -rf "$work"
  return $status
}

if [ $# -gt 0 ]; then ids=("$@"); else ids=("$ID1" "$ID2"); fi
rc=0
for who in "${ids[@]}"; do mint_token "$who" || rc=1; done
exit $rc
