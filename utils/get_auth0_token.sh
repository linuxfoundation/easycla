#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
#
# Fully automatic Auth0 access token generator - no browser, no prompts.
# Performs the classic Auth0 cross-origin login flow (authorize -> login page
# -> /usernamepassword/login -> /login/callback -> /authorize/resume -> PKCE
# code exchange) using curl only, reading credentials from *.secret files.
#
# Credentials file (gitignored): utils/auth0-dev.secret / utils/auth0-prod.secret
# (template: utils/auth0.secret.example), parsed as simple KEY=VALUE lines - never sourced:
#   AUTH0_USERNAME=someuser
#   AUTH0_PASSWORD=somepassword
# Optional overrides in the same file: AUTH0_DOMAIN, AUTH0_CLIENT_ID,
# AUTH0_AUDIENCE, AUTH0_TENANT, AUTH0_REDIRECT_URI.
#
# Output: token written to utils/auth0-dev.token.secret (or auth0-prod.token.secret)
# and printed on stdout, so both work:
#   TOKEN=$(./utils/get_auth0_token.sh dev)
#   TOKEN=$(cat utils/auth0-dev.token.secret)
#
# Usage examples:
#   ./utils/get_auth0_token.sh              # dev token (default stage)
#   ./utils/get_auth0_token.sh dev          # dev token -> utils/auth0-dev.token.secret
#   ./utils/get_auth0_token.sh prod         # prod token -> utils/auth0-prod.token.secret
#   TOKEN=$(./utils/get_auth0_token.sh dev) # capture directly, diagnostics go to stderr
#   DEBUG=1 ./utils/get_auth0_token.sh dev  # verbose step-by-step diagnostics
#   ./utils/get_auth0_token.sh dev && curl -s -H "Authorization: Bearer $(cat utils/auth0-dev.token.secret)" \
#     "https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/my-clas" | jq .
#
# Token lifetimes (Auth0 client settings): dev ~1h, prod ~3h.

set -euo pipefail

STAGE="${1:-dev}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

case "$STAGE" in
  dev)
    AUTH0_DOMAIN="linuxfoundation-dev.auth0.com"
    AUTH0_CLIENT_ID="G5CNCTp6X5Z1HizkotPHm6Ug11oGr2Eo"
    AUTH0_AUDIENCE="https://api-gw.dev.platform.linuxfoundation.org/"
    AUTH0_TENANT="linuxfoundation-dev"
    ;;
  prod)
    AUTH0_DOMAIN="sso.linuxfoundation.org"
    AUTH0_CLIENT_ID="DoMcTpihSo3is7hfGngHz7phw7kC6daw"
    AUTH0_AUDIENCE="https://api-gw.platform.linuxfoundation.org/"
    AUTH0_TENANT="linuxfoundation"
    ;;
  *)
    echo "usage: $0 [dev|prod]" >&2
    exit 1
    ;;
esac

AUTH0_REDIRECT_URI="http://localhost:55001/callback"
SECRET_FILE="$SCRIPT_DIR/auth0-$STAGE.secret"
TOKEN_FILE="$SCRIPT_DIR/auth0-$STAGE.token.secret"

if [ ! -f "$SECRET_FILE" ]; then
  {
    echo "error: credentials file $SECRET_FILE not found"
    echo "create it (gitignored, see utils/auth0.secret.example) with:"
    echo "  AUTH0_USERNAME=youruser"
    echo "  AUTH0_PASSWORD=yourpassword"
  } >&2
  exit 1
fi

# parse KEY=VALUE lines instead of sourcing, so the secret file can never execute code
secret_get() { sed -n "s/^$1=//p" "$SECRET_FILE" | tail -1 | tr -d '\r'; }

AUTH0_USERNAME="$(secret_get AUTH0_USERNAME)"
AUTH0_PASSWORD="$(secret_get AUTH0_PASSWORD)"
for key in AUTH0_DOMAIN AUTH0_CLIENT_ID AUTH0_AUDIENCE AUTH0_TENANT AUTH0_REDIRECT_URI; do
  value="$(secret_get "$key")"
  [ -n "$value" ] && eval "$key=\"\$value\""
done

if [ -z "${AUTH0_USERNAME:-}" ] || [ -z "${AUTH0_PASSWORD:-}" ]; then
  echo "error: AUTH0_USERNAME/AUTH0_PASSWORD not set in $SECRET_FILE" >&2
  exit 1
fi

dbg() { [ -n "${DEBUG:-}" ] && echo "$@" >&2 || true; }

umask 077
JAR="$(mktemp)"
trap 'rm -f "$JAR"' EXIT

b64url() { openssl base64 -A | tr '+/' '-_' | tr -d '='; }
urlenc() { python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$1"; }

STATE="$(head -c 18 /dev/urandom | b64url)"
VERIFIER="$(head -c 48 /dev/urandom | b64url)"
CHALLENGE="$(printf %s "$VERIFIER" | openssl dgst -sha256 -binary | b64url)"

AUTHORIZE_URL="https://$AUTH0_DOMAIN/authorize?client_id=$AUTH0_CLIENT_ID&response_type=code&redirect_uri=$(urlenc "$AUTH0_REDIRECT_URI")&scope=$(urlenc "openid profile email access:api")&audience=$(urlenc "$AUTH0_AUDIENCE")&state=$STATE&code_challenge=$CHALLENGE&code_challenge_method=S256"

# step 1: /authorize -> 302 to the hosted login page carrying the interaction state
LOGIN_URL="$(curl -sS -c "$JAR" -o /dev/null -w '%{redirect_url}' "$AUTHORIZE_URL")"
dbg "step1 authorize -> $LOGIN_URL"
case "$LOGIN_URL" in
  *state=*) ;;
  *) echo "error: /authorize did not redirect to the login page (check client/audience settings)" >&2; exit 1 ;;
esac
LOGIN_STATE="$(printf %s "$LOGIN_URL" | sed -n 's/.*[?&]state=\([^&]*\).*/\1/p')"

# step 2: fetch the login page to obtain the _csrf cookie
curl -sS -b "$JAR" -c "$JAR" -o /dev/null "$LOGIN_URL"
CSRF="$(awk '$6=="_csrf"{print $7}' "$JAR" | tail -1)"
dbg "step2 login page fetched, csrf present: $([ -n "$CSRF" ] && echo yes || echo no)"

# step 3: same-origin credentials POST -> WS-Fed self-posting form
# credentials reach python via the environment and curl via stdin - never via argv
UPL_BODY="$(A0_CLIENT_ID="$AUTH0_CLIENT_ID" A0_REDIRECT_URI="$AUTH0_REDIRECT_URI" A0_TENANT="$AUTH0_TENANT" \
  A0_AUDIENCE="$AUTH0_AUDIENCE" A0_STATE="$LOGIN_STATE" A0_USERNAME="$AUTH0_USERNAME" \
  A0_PASSWORD="$AUTH0_PASSWORD" A0_CSRF="$CSRF" python3 << 'PYEOF'
import json
import os

env = os.environ
print(json.dumps({
    "client_id": env["A0_CLIENT_ID"],
    "redirect_uri": env["A0_REDIRECT_URI"],
    "tenant": env["A0_TENANT"],
    "response_type": "code",
    "scope": "openid profile email access:api",
    "audience": env["A0_AUDIENCE"],
    "state": env["A0_STATE"],
    "username": env["A0_USERNAME"],
    "password": env["A0_PASSWORD"],
    "connection": "Username-Password-Authentication",
    "protocol": "oauth2",
    "popup_options": {},
    "sso": True,
    "_csrf": env["A0_CSRF"],
    "_intstate": "deprecated",
}))
PYEOF
)"
UPL_HTML="$(printf %s "$UPL_BODY" | curl -sS -b "$JAR" -c "$JAR" -X POST "https://$AUTH0_DOMAIN/usernamepassword/login" \
  -H "Content-Type: application/json" -H "Origin: https://$AUTH0_DOMAIN" -H "Referer: $LOGIN_URL" \
  --data @-)"
if ! printf %s "$UPL_HTML" | grep -q 'name="wresult"'; then
  echo "error: login failed: $(printf %s "$UPL_HTML" | head -c 300)" >&2
  exit 1
fi
dbg "step3 credentials accepted"

# step 4: post the WS-Fed form back to /login/callback -> 302 /authorize/resume
# the page HTML feeds python via stdin (program via -c, so stdin stays the pipe)
CALLBACK_FORM="$(printf %s "$UPL_HTML" | python3 -c '
import html
import re
import sys
import urllib.parse

page = sys.stdin.read()

def field(name):
    match = re.search("name=\"" + name + "\"\\s+value=\"([^\"]*)\"", page)
    return html.unescape(match.group(1)) if match else ""

if not field("wresult"):
    sys.exit("error: no wresult field on the WS-Fed page - login flow changed?")
print(urllib.parse.urlencode({"wa": field("wa"), "wresult": field("wresult"), "wctx": field("wctx")}))
')"
RESUME_URL="$(printf %s "$CALLBACK_FORM" | curl -sS -b "$JAR" -c "$JAR" -o /dev/null -w '%{redirect_url}' -X POST "https://$AUTH0_DOMAIN/login/callback" \
  -H "Content-Type: application/x-www-form-urlencoded" -H "Origin: https://$AUTH0_DOMAIN" --data @-)"
dbg "step4 login callback -> $RESUME_URL"
case "$RESUME_URL" in
  *"/authorize/resume"*) ;;
  *) echo "error: unexpected /login/callback redirect: $RESUME_URL" >&2; exit 1 ;;
esac

# step 5: resume -> 302 to redirect_uri with ?code= (no local listener needed)
FINAL_URL="$(curl -sS -b "$JAR" -c "$JAR" -o /dev/null -w '%{redirect_url}' "$RESUME_URL")"
CODE="$(printf %s "$FINAL_URL" | sed -n 's/.*[?&]code=\([^&]*\).*/\1/p')"
dbg "step5 resume -> code present: $([ -n "$CODE" ] && echo yes || echo no)"
if [ -z "$CODE" ]; then
  echo "error: no authorization code returned: $FINAL_URL" >&2
  exit 1
fi

# step 6: PKCE code exchange (code and verifier via stdin, not argv)
TOKEN_JSON="$(A0_CLIENT_ID="$AUTH0_CLIENT_ID" A0_CODE="$CODE" A0_REDIRECT_URI="$AUTH0_REDIRECT_URI" A0_VERIFIER="$VERIFIER" python3 << 'PYEOF' | curl -sS -X POST "https://$AUTH0_DOMAIN/oauth/token" -H "Content-Type: application/json" --data @-
import json
import os

env = os.environ
print(json.dumps({
    "grant_type": "authorization_code",
    "client_id": env["A0_CLIENT_ID"],
    "code": env["A0_CODE"],
    "redirect_uri": env["A0_REDIRECT_URI"],
    "code_verifier": env["A0_VERIFIER"],
}))
PYEOF
)"
ACCESS_TOKEN="$(printf %s "$TOKEN_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("access_token",""))')"
if [ -z "$ACCESS_TOKEN" ]; then
  echo "error: token exchange failed: $(printf %s "$TOKEN_JSON" | head -c 300)" >&2
  exit 1
fi

printf '%s\n' "$ACCESS_TOKEN" > "$TOKEN_FILE"
chmod 600 "$TOKEN_FILE"
EXPIRES_IN="$(printf %s "$TOKEN_JSON" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("expires_in",""))')"
echo "token for stage '$STAGE' saved to $TOKEN_FILE (expires in ${EXPIRES_IN}s)" >&2
printf '%s\n' "$ACCESS_TOKEN"
