#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
#
# Local-only helper (not deployed): mints an LFX user access token for the EasyCLA API gateway
# with the Auth0 Device Code flow of the LFX CLI's native application (no client secret), then
# keeps minting from the stored refresh token without a browser.
#
# Secrets are never embedded here; every confidential value comes from gitignored *.secret files:
#   utils/lfx-<stage>-client-id.secret      native (device-code) Auth0 client id for the stage
#   utils/lfx-<stage>-refresh.token.secret  refresh token written by `login`, read by every later run
#   utils/auth0-<stage>.token.secret        access token written on success (same file the other
#                                           utils/ harnesses read), also printed to stdout
#
# Usage:
#   ./utils/get_lfx_token.sh dev|prod login   # first time per stage: device code; the consent is
#                                             # completed headlessly by utils/lfx_device_consent.mjs
#                                             # with utils/auth0-<stage>.secret (set LFX_MANUAL_CONSENT=1
#                                             # to approve the printed URL + code in a browser instead)
#   ./utils/get_lfx_token.sh dev|prod         # later: refresh -> new access token (no browser)
#   TOKEN=$(./utils/get_lfx_token.sh prod)
# Optional overrides via environment: LFX_AUDIENCE, LFX_SCOPE, LFX_AUTH0_DOMAIN. Secrets travel only through
# the environment and stdin of child processes, never through command-line arguments.
set -euo pipefail

STAGE="${1:-}"
MODE="${2:-refresh}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

case "$STAGE" in
  dev)
    AUTH0_DOMAIN="${LFX_AUTH0_DOMAIN:-linuxfoundation-dev.auth0.com}"
    AUDIENCE="${LFX_AUDIENCE:-https://api-gw.dev.platform.linuxfoundation.org/}"
    ;;
  prod)
    AUTH0_DOMAIN="${LFX_AUTH0_DOMAIN:-sso.linuxfoundation.org}"
    AUDIENCE="${LFX_AUDIENCE:-https://api-gw.platform.linuxfoundation.org/}"
    ;;
  *)
    echo "usage: $0 dev|prod [login|refresh]" >&2
    exit 1
    ;;
esac
case "$MODE" in login|refresh) ;; *) echo "usage: $0 dev|prod [login|refresh]" >&2; exit 1 ;; esac

SCOPE="${LFX_SCOPE:-openid profile email offline_access access:api}"
CLIENT_ID_FILE="$SCRIPT_DIR/lfx-$STAGE-client-id.secret"
REFRESH_FILE="$SCRIPT_DIR/lfx-$STAGE-refresh.token.secret"
TOKEN_FILE="$SCRIPT_DIR/auth0-$STAGE.token.secret"

[ -f "$CLIENT_ID_FILE" ] || { echo "error: $CLIENT_ID_FILE not found (device-code client id for $STAGE)" >&2; exit 1; }
CLIENT_ID="$(tr -d '\r\n' < "$CLIENT_ID_FILE")"
[ -n "$CLIENT_ID" ] || { echo "error: $CLIENT_ID_FILE is empty" >&2; exit 1; }

CURL=(curl -sS --fail-with-body --max-time 30)

json_field() { # json_field <key> : prints the string/number value of a top-level key from stdin JSON
  python3 -c 'import json,sys; d=json.load(sys.stdin); v=d.get(sys.argv[1]); print("" if v is None else v)' "$1"
}

save_secret() { # save_secret <file> <value>
  umask 077
  printf '%s\n' "$2" > "$1"
  chmod 600 "$1"
}

check_token() { # check_token : verifies audience/expiry of $LFX_CHECK_TOKEN (environment, never argv), prints expiry info to stderr
  LFX_CHECK_TOKEN="$1" LFX_CHECK_AUDIENCE="$AUDIENCE" python3 - <<'PY'
import base64, json, os, sys, time
token, audience = os.environ["LFX_CHECK_TOKEN"], os.environ["LFX_CHECK_AUDIENCE"]
parts = token.split(".")
if len(parts) != 3:
    sys.exit("error: the response did not contain a JWT access token")
payload = parts[1] + "=" * (-len(parts[1]) % 4)
claims = json.loads(base64.urlsafe_b64decode(payload))
aud = claims.get("aud", [])
aud = [aud] if isinstance(aud, str) else aud
if audience not in aud:
    sys.exit("error: token audience %s does not include %s" % (aud, audience))
left = int(claims.get("exp", 0) - time.time())
if left <= 0:
    sys.exit("error: token already expired")
print("token ok: expires in %d min, subject %s" % (left // 60, claims.get("sub", "?")), file=sys.stderr)
PY
}

form_body() { # form_body : url-encodes every LFX_FORM_<name> environment variable into a form body on stdout
  python3 -c 'import os, urllib.parse; print(urllib.parse.urlencode({k[9:]: v for k, v in os.environ.items() if k.startswith("LFX_FORM_")}))'
}

post_form() { # post_form <url> : POSTs the LFX_FORM_* body read from stdin (never from argv), prints the JSON body
  form_body | "${CURL[@]}" -X POST "$1" -H "Content-Type: application/x-www-form-urlencoded" --data @-
}

if [ "$MODE" = "login" ]; then
  DEVICE="$(LFX_FORM_client_id="$CLIENT_ID" LFX_FORM_scope="$SCOPE" LFX_FORM_audience="$AUDIENCE" \
    post_form "https://$AUTH0_DOMAIN/oauth/device/code")"
  DEVICE_CODE="$(printf '%s' "$DEVICE" | json_field device_code)"
  USER_CODE="$(printf '%s' "$DEVICE" | json_field user_code)"
  VERIFY_URL="$(printf '%s' "$DEVICE" | json_field verification_uri_complete)"
  INTERVAL="$(printf '%s' "$DEVICE" | json_field interval)"
  EXPIRES="$(printf '%s' "$DEVICE" | json_field expires_in)"
  [ -n "$DEVICE_CODE" ] || { echo "error: no device code in the response" >&2; exit 1; }
  if [ -z "${LFX_MANUAL_CONSENT:-}" ] && [ -f "$SCRIPT_DIR/auth0-$STAGE.secret" ] && command -v node >/dev/null 2>&1; then
    echo "Completing the device consent headlessly (utils/lfx_device_consent.mjs)..." >&2
    if ! LFX_VERIFY_URL="$VERIFY_URL" LFX_USER_CODE="$USER_CODE" node "$SCRIPT_DIR/lfx_device_consent.mjs" "$STAGE"; then
      echo "Headless consent did not finish - open in a browser and approve: $VERIFY_URL (user code $USER_CODE)" >&2
    fi
  else
    echo "Open in a browser (any device) and approve: $VERIFY_URL" >&2
    echo "User code: $USER_CODE  (valid for $EXPIRES s)" >&2
  fi
  echo "Waiting for the approval..." >&2
  INTERVAL="${INTERVAL:-5}"
  DEADLINE=$(( $(date +%s) + ${EXPIRES:-900} ))
  while :; do
    sleep "$INTERVAL"
    BODY="$(LFX_FORM_grant_type="urn:ietf:params:oauth:grant-type:device_code" LFX_FORM_device_code="$DEVICE_CODE" \
      LFX_FORM_client_id="$CLIENT_ID" post_form "https://$AUTH0_DOMAIN/oauth/token" 2>/dev/null || true)"
    ERR="$(printf '%s' "$BODY" | json_field error 2>/dev/null || true)"
    case "$ERR" in
      "") break ;;
      authorization_pending) ;;
      slow_down) INTERVAL=$((INTERVAL + 5)) ;;
      *) echo "error: device flow failed: $ERR" >&2; exit 1 ;;
    esac
    [ "$(date +%s)" -lt "$DEADLINE" ] || { echo "error: the device code expired before approval" >&2; exit 1; }
  done
else
  [ -f "$REFRESH_FILE" ] || { echo "error: $REFRESH_FILE not found - run: $0 $STAGE login" >&2; exit 1; }
  REFRESH_TOKEN="$(tr -d '\r\n' < "$REFRESH_FILE")"
  BODY="$(LFX_FORM_grant_type="refresh_token" LFX_FORM_client_id="$CLIENT_ID" LFX_FORM_refresh_token="$REFRESH_TOKEN" \
    post_form "https://$AUTH0_DOMAIN/oauth/token")"
fi

ACCESS_TOKEN="$(printf '%s' "$BODY" | json_field access_token)"
[ -n "$ACCESS_TOKEN" ] || { echo "error: no access token in the response ($(printf '%s' "$BODY" | json_field error_description))" >&2; exit 1; }
check_token "$ACCESS_TOKEN"
NEW_REFRESH="$(printf '%s' "$BODY" | json_field refresh_token)"
if [ -n "$NEW_REFRESH" ]; then
  save_secret "$REFRESH_FILE" "$NEW_REFRESH"
  echo "refresh token saved to $REFRESH_FILE" >&2
elif [ "$MODE" = "login" ]; then
  echo "warning: no refresh token issued - every later run will need a new login" >&2
fi
save_secret "$TOKEN_FILE" "$ACCESS_TOKEN"
echo "access token saved to $TOKEN_FILE" >&2
printf '%s\n' "$ACCESS_TOKEN"
