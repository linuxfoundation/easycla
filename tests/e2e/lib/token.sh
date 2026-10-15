#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
#
# Mint (or reuse) api-gw access tokens for the suite identities without modifying utils/get_auth0_token.sh:
# the script is staged in a private temp dir together with the identity's credentials file renamed to the
# name it expects (auth0-<env>.secret) and the client-id file, then the token is moved to
#   secrets/auth0-<env>-<identity>.token.secret
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

mint_token() {
  local identity=$1 cached left work status=0
  cached="$(e2e_token "$identity")"
  if [ -n "$cached" ] && [ "${FORCE:-0}" != "1" ]; then
    left="$(token_seconds_left "$cached")"
    if [ "$left" -gt 600 ]; then e2e_log "token for $identity still valid for ${left}s - reusing"; return 0; fi
  fi
  e2e_require_secret "auth0-$E2E_ENV-$identity"
  work="$(mktemp -d "${TMPDIR:-/tmp}/e2e-token.XXXXXX")"; chmod 700 "$work"
  cp -p "$E2E_REPO_ROOT/utils/get_auth0_token.sh" "$work/get_auth0_token.sh"
  cp "$(e2e_secret_path "auth0-$E2E_ENV-$identity")" "$work/auth0-$E2E_ENV.secret"
  chmod 600 "$work/auth0-$E2E_ENV.secret"
  local cid; cid="$(e2e_secret_path "auth0-$E2E_ENV-client-id")"
  [ -f "$cid" ] || cid="$E2E_REPO_ROOT/utils/auth0-$E2E_ENV-client-id.secret"
  if [ -f "$cid" ]; then cp "$cid" "$work/auth0-$E2E_ENV-client-id.secret"; chmod 600 "$work/auth0-$E2E_ENV-client-id.secret"; fi
  e2e_log "minting $E2E_ENV token for $identity"
  if ! (cd "$work" && ./get_auth0_token.sh "$E2E_ENV" >/dev/null 2>"$work/mint.err"); then
    status=1; e2e_log "token minting FAILED for $identity: $(e2e_redact 300 < "$work/mint.err")"
  else
    cp "$work/auth0-$E2E_ENV.token.secret" "$(e2e_secret_path "auth0-$E2E_ENV-$identity.token")"
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
