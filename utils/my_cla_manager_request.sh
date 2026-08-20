#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# Calls POST /v4/my-clas/{signatureID}/cla-manager-requests through lfx-gateway and reports the HTTP status and total time.
# SIGNATURE_ID (or 1st arg): the ECLA signature ID - required.
# REQUEST_TYPE (or 2nd arg): removal (default) | approval.
# RECIPIENTS (or 3rd arg): comma-separated CLA manager LF usernames (must come from ./utils/my_cla_managers.sh output; empty only when no manager resolves).
# MESSAGE: optional message included in the notification email.
# TOKEN: bearer access token (env, or ./my_clas.token.secret / ./auth0.token.secret). Get one with ~/get_oauth_token.sh (dev) or ~/get_oauth_token_prod.sh (prod).
# STAGE: dev (default) | test | staging | prod - selects the api-gw host.
# Identity params (each comma-separated, repeated in the query): LF_USERNAME EMAIL SECONDARY_EMAIL GITHUB_ID GITHUB_USERNAME GITLAB_ID GITLAB_USERNAME GERRIT_USERNAME
# Local mode (against a standalone backend, bypassing the gateway): set PRINCIPAL to the token username and ADMIN=true|false (or pass a raw base64 X_ACL). Defaults API_URL to http://localhost:8080.
# Examples:
#   SIGNATURE_ID=3c1e5d7a-... RECIPIENTS=manager1,manager2 MESSAGE='please remove me' ./utils/my_cla_manager_request.sh
#   ./utils/my_cla_manager_request.sh 3c1e5d7a-... approval manager1

[ -z "$SIGNATURE_ID" ] && SIGNATURE_ID="$1"
[ -z "$REQUEST_TYPE" ] && REQUEST_TYPE="${2:-removal}"
[ -z "$RECIPIENTS" ] && RECIPIENTS="$3"
if [ -z "$SIGNATURE_ID" ]
then
  echo "$0: SIGNATURE_ID (or 1st arg) is required"
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
    [ -f ./my_clas.token.secret ] && TOKEN="$(cat ./my_clas.token.secret)"
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

query=""
add_param() {
  local name="$1" values="$2"
  [ -z "$values" ] && return
  local IFS=,
  for v in $values
  do
    [ -n "$v" ] && query="${query}${query:+&}${name}=${v}"
  done
}
add_param lfUsername "$LF_USERNAME"
add_param email "$EMAIL"
add_param secondaryEmail "$SECONDARY_EMAIL"
add_param githubId "$GITHUB_ID"
add_param githubUsername "$GITHUB_USERNAME"
add_param gitlabId "$GITLAB_ID"
add_param gitlabUsername "$GITLAB_USERNAME"
add_param gerritUsername "$GERRIT_USERNAME"

URL="${API_URL}/v4/my-clas/${SIGNATURE_ID}/cla-manager-requests${query:+?}${query}"

recipients_json="[]"
if [ -n "$RECIPIENTS" ]
then
  recipients_json="[$(echo "$RECIPIENTS" | sed 's/[^,]*/"&"/g')]"
fi
payload="$(printf '{"requestType":"%s","recipients":%s' "$REQUEST_TYPE" "$recipients_json")"
if [ -n "$MESSAGE" ]
then
  if command -v jq >/dev/null 2>&1
  then
    payload="${payload},\"message\":$(printf '%s' "$MESSAGE" | jq -Rs .)"
  else
    payload="${payload},\"message\":\"${MESSAGE}\""
  fi
fi
payload="${payload}}"

if [ -n "$DEBUG" ]
then
  echo "curl -sS -XPOST ${auth[0]} '${auth[1]%%:*}: <redacted>' -H 'Content-Type: application/json' -d '${payload}' '${URL}'"
fi

body="$(mktemp)"
timing="$(curl -sS -XPOST "${auth[@]}" -H "Content-Type: application/json" -d "$payload" -w '%{http_code} %{time_total}' -o "$body" "$URL")"
if command -v jq >/dev/null 2>&1
then
  jq -r '.' < "$body" 2>/dev/null || cat "$body"
else
  cat "$body"
fi
echo
echo "HTTP ${timing% *} in ${timing#* }s"
rm -f "$body"
