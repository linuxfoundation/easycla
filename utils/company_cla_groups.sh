#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# Calls GET /v4/company/external/{companySFID}/cla-groups through lfx-gateway and reports the HTTP status and total time.
# COMPANY_SFID (or 1st arg): the company external Salesforce ID - required.
# TOKEN: bearer access token (env, or ./company_cla_groups.token.secret / ./auth0.token.secret). Get one with ~/get_oauth_token.sh (dev) or ~/get_oauth_token_prod.sh (prod).
# STAGE: dev (default) | test | staging | prod - selects the api-gw host.
# Local mode (against a standalone backend, bypassing the gateway): set PRINCIPAL to the token username and ADMIN=true|false (or pass a raw base64 X_ACL). Defaults API_URL to http://localhost:8080.
# Examples:
#   COMPANY_SFID=0014100000Te0G7AAJ ./utils/company_cla_groups.sh
#   PRINCIPAL=lgryglicki ADMIN=true ./utils/company_cla_groups.sh 0014100000Te0G7AAJ

[ -z "$COMPANY_SFID" ] && COMPANY_SFID="$1"
if [ -z "$COMPANY_SFID" ]
then
  echo "$0: COMPANY_SFID (or 1st arg) is required"
  exit 3
fi

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
    [ -f ./company_cla_groups.token.secret ] && TOKEN="$(cat ./company_cla_groups.token.secret)"
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

URL="${API_URL}/v4/company/external/${COMPANY_SFID}/cla-groups"

if [ -n "$DEBUG" ]
then
  echo "curl -sS -XGET '${auth[0]}: <redacted>' -H 'Content-Type: application/json' '${URL}'"
fi

body="$(mktemp)"
timing="$(curl -sS -XGET "${auth[@]}" -H "Content-Type: application/json" -w '%{http_code} %{time_total}' -o "$body" "$URL")"
if command -v jq >/dev/null 2>&1
then
  jq -r '.' < "$body" 2>/dev/null || cat "$body"
else
  cat "$body"
fi
echo
echo "HTTP ${timing% *} in ${timing#* }s"
rm -f "$body"
