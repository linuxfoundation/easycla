#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# Calls POST /v4/self-serve/request-corporate-signature through lfx-gateway and reports the HTTP status and total time.
# WARNING: a 200 response creates a real DocuSign envelope (and sends a signing email when SEND_AS_EMAIL=true) in the targeted environment.
# PROJECT_SFID (or 1st arg) and COMPANY_SFID (or 2nd arg): required.
# AUTHORITY_ACKED / EMBARGO_ACKED: default true - set to false to probe the attestation 400 (no DocuSign side effects).
# SIGNING_ENTITY_NAME, SEND_AS_EMAIL, AUTHORITY_NAME, AUTHORITY_EMAIL, RETURN_URL: optional corporate-signature-input passthrough fields.
# TOKEN: bearer access token (env, or ./self_serve_request_corporate_signature.token.secret / ./auth0.token.secret). Get one with ~/get_oauth_token.sh (dev) or ~/get_oauth_token_prod.sh (prod).
# STAGE: dev (default) | test | staging | prod - selects the api-gw host.
# Local mode (against a standalone backend, bypassing the gateway): set PRINCIPAL to the token username and ADMIN=true|false (or pass a raw base64 X_ACL). Defaults API_URL to http://localhost:8080.
# Examples:
#   AUTHORITY_ACKED=false PROJECT_SFID=a0941000005ouJFAAY COMPANY_SFID=0014100000Te0G7AAJ ./utils/self_serve_request_corporate_signature.sh
#   PRINCIPAL=lgryglicki ./utils/self_serve_request_corporate_signature.sh a0941000005ouJFAAY 0014100000Te0G7AAJ

[ -z "$PROJECT_SFID" ] && PROJECT_SFID="$1"
[ -z "$COMPANY_SFID" ] && COMPANY_SFID="$2"
if [ -z "$PROJECT_SFID" ] || [ -z "$COMPANY_SFID" ]
then
  echo "$0: PROJECT_SFID (or 1st arg) and COMPANY_SFID (or 2nd arg) are required"
  exit 3
fi
[ -z "$AUTHORITY_ACKED" ] && AUTHORITY_ACKED=true
[ -z "$EMBARGO_ACKED" ] && EMBARGO_ACKED=true

if [ -n "$PRINCIPAL" ] && [ -z "$X_ACL" ]
then
  admin=false
  [ "$ADMIN" = "true" ] && admin=true
  X_ACL="$(printf '{"user_name":"%s","email":"%s","isAdmin":%s,"allowed":true}' "$PRINCIPAL" "${PRINCIPAL_EMAIL:-$PRINCIPAL}" "$admin" | base64 | tr -d '\n')"
fi

if [ -n "$X_ACL" ]
then
  auth=(-H "X-ACL: ${X_ACL}")
  [ -z "$API_URL" ] && API_URL="http://localhost:${PORT:-8080}"
else
  if [ -z "$TOKEN" ]
  then
    [ -f ./self_serve_request_corporate_signature.token.secret ] && TOKEN="$(cat ./self_serve_request_corporate_signature.token.secret)"
  fi
  if [ -z "$TOKEN" ]
  then
    [ -f ./auth0.token.secret ] && TOKEN="$(cat ./auth0.token.secret)"
  fi
  if [ -z "$TOKEN" ]
  then
    echo "$0: TOKEN not set - run ~/get_oauth_token.sh (dev) or ~/get_oauth_token_prod.sh (prod) and export TOKEN (or use PRINCIPAL=... for local mode)"
    exit 1
  fi
  auth=(-H "Authorization: Bearer ${TOKEN}")
  if [ -z "$STAGE" ]
  then
    STAGE=dev
  fi
  case "$STAGE" in
    prod) GW="https://api-gw.platform.linuxfoundation.org" ;;
    staging) GW="https://api-gw.staging.platform.linuxfoundation.org" ;;
    test) GW="https://api-gw.test.platform.linuxfoundation.org" ;;
    dev) GW="https://api-gw.dev.platform.linuxfoundation.org" ;;
    *) echo "$0: unknown STAGE '$STAGE'"; exit 2 ;;
  esac
  [ -z "$API_URL" ] && API_URL="${GW}/cla-service"
fi

URL="${API_URL}/v4/self-serve/request-corporate-signature"

payload="{\"project_sfid\":\"${PROJECT_SFID}\",\"company_sfid\":\"${COMPANY_SFID}\",\"authority_acked\":${AUTHORITY_ACKED},\"embargo_acked\":${EMBARGO_ACKED}"
[ -n "$SIGNING_ENTITY_NAME" ] && payload="${payload},\"signing_entity_name\":\"${SIGNING_ENTITY_NAME}\""
[ "$SEND_AS_EMAIL" = "true" ] && payload="${payload},\"send_as_email\":true"
[ -n "$AUTHORITY_NAME" ] && payload="${payload},\"authority_name\":\"${AUTHORITY_NAME}\""
[ -n "$AUTHORITY_EMAIL" ] && payload="${payload},\"authority_email\":\"${AUTHORITY_EMAIL}\""
[ -n "$RETURN_URL" ] && payload="${payload},\"return_url\":\"${RETURN_URL}\""
payload="${payload}}"

if [ -n "$DEBUG" ]
then
  echo "curl -sS -XPOST '${auth[0]}: <redacted>' -H 'Content-Type: application/json' -d '${payload}' '${URL}'"
fi

body="$(mktemp)"
timing="$(curl -sS -XPOST "${auth[@]}" -H "Content-Type: application/json" -d "${payload}" -w '%{http_code} %{time_total}' -o "$body" "$URL")"
if command -v jq >/dev/null 2>&1
then
  jq -r '.' < "$body" 2>/dev/null || cat "$body"
else
  cat "$body"
fi
echo
echo "HTTP ${timing% *} in ${timing#* }s"
rm -f "$body"
