#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# Calls the Self Serve sign APIs and reports the HTTP status and total time.
# Default: POST /v4/self-serve/prepare-sign - confirms the given identity belongs to the token's LFID, creates the EasyCLA user record when missing, and returns the Contributor Console sign URL.
# SIGN_ICLA=1: POST /v4/request-individual-signature the way the Contributor Console does after a prepare - needs USER_ID and CLA_GROUP_ID, prints the DocuSign sign_url.
# CALLBACK=<userID>: POST /v4/signed/self-serve/individual/{user_id} - the DocuSign callback of a Self Serve started ICLA (XML body from PAYLOAD, default ./docusign_payload.xml); no token, DocuSign HMAC applies.
# TOKEN: bearer access token (env, or ./prepare_sign.token.secret / ./my_clas.token.secret / ./auth0.token.secret). Get one with ~/get_oauth_token.sh (dev) or ~/get_oauth_token_prod.sh (prod).
# STAGE: dev (default) | test | staging | prod - selects the api-gw host.
# CLA_GROUP_ID (or 1st arg): the CLA Group UUID to sign - required.
# RETURN_URL: where the Console sends the contributor once signing completes - the Self Serve My CLAs page; required for prepare-sign.
# Identity params (a single value each): LF_USERNAME EMAIL GITHUB_ID GITHUB_USERNAME GITLAB_ID GITLAB_USERNAME GERRIT_USERNAME
# Local mode (against a standalone backend, bypassing the gateway - lets you test the non-admin ownership enforcement): set PRINCIPAL to the token username and ADMIN=true|false (or pass a raw base64 X_ACL). Defaults API_URL to http://localhost:8080.
# Examples:
#   CLA_GROUP_ID=01af041c-... GITHUB_ID=2469783 GITHUB_USERNAME=jdoe ./utils/prepare_sign.sh
#   CLA_GROUP_ID=01af041c-... RETURN_URL=https://openprofile.dev/my-clas ./utils/prepare_sign.sh          # dev deployed
#   STAGE=prod TOKEN="$(~/get_oauth_token_prod.sh)" CLA_GROUP_ID=01af041c-... ./utils/prepare_sign.sh      # prod deployed
#   PRINCIPAL=lgryglicki ADMIN=false CLA_GROUP_ID=01af041c-... GITHUB_ID=2469783 ./utils/prepare_sign.sh   # local
#   SIGN_ICLA=1 CLA_GROUP_ID=01af041c-... USER_ID=6c2d5a11-... ./utils/prepare_sign.sh                       # console leg: prepare -> request -> sign_url
#   CALLBACK=6c2d5a11-... PAYLOAD=./docusign_payload.xml API_URL=http://localhost:8080 ./utils/prepare_sign.sh

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
    [ -f ./prepare_sign.token.secret ] && TOKEN="$(cat ./prepare_sign.token.secret)"
  fi
  if [ -z "$TOKEN" ]
  then
    [ -f ./my_clas.token.secret ] && TOKEN="$(cat ./my_clas.token.secret)"
  fi
  if [ -z "$TOKEN" ]
  then
    [ -f ./auth0.token.secret ] && TOKEN="$(cat ./auth0.token.secret)"
  fi
  if [ -z "$TOKEN" ] && [ -z "$CALLBACK" ]
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

body="$(mktemp)"
if [ -n "$CALLBACK" ]
then
  [ -z "$PAYLOAD" ] && PAYLOAD="./docusign_payload.xml"
  if [ ! -f "$PAYLOAD" ]
  then
    echo "$0: PAYLOAD file '$PAYLOAD' not found - DocuSign callback needs the signed envelope XML"
    rm -f "$body"
    exit 3
  fi
  URL="${API_URL}/v4/signed/self-serve/individual/${CALLBACK}"
  [ -n "$DEBUG" ] && echo "curl -sS -XPOST -H 'Content-Type: text/xml' --data-binary @${PAYLOAD} '${URL}'"
  timing="$(curl -sS -XPOST -H "Content-Type: text/xml" --data-binary "@${PAYLOAD}" -w '%{http_code} %{time_total}' -o "$body" "$URL")"
else
  [ -z "$CLA_GROUP_ID" ] && CLA_GROUP_ID="$1"
  if [ -z "$CLA_GROUP_ID" ]
  then
    echo "$0: CLA_GROUP_ID not set - pass the CLA Group UUID to sign as CLA_GROUP_ID or as the first argument"
    rm -f "$body"
    exit 4
  fi
  if [ -z "$SIGN_ICLA" ] && [ -z "$RETURN_URL" ]
  then
    echo "$0: RETURN_URL not set - prepare-sign requires the URL the Contributor Console returns the contributor to, e.g. RETURN_URL=https://openprofile.dev/my-clas"
    rm -f "$body"
    exit 7
  fi
  if ! command -v jq >/dev/null 2>&1
  then
    echo "$0: jq is required to build the request body"
    rm -f "$body"
    exit 5
  fi
  payload="$(jq -nc \
    --arg claGroupId "$CLA_GROUP_ID" \
    --arg returnUrl "$RETURN_URL" \
    --arg lfUsername "$LF_USERNAME" \
    --arg email "$EMAIL" \
    --arg githubId "$GITHUB_ID" \
    --arg githubUsername "$GITHUB_USERNAME" \
    --arg gitlabId "$GITLAB_ID" \
    --arg gitlabUsername "$GITLAB_USERNAME" \
    --arg gerritUsername "$GERRIT_USERNAME" \
    '{claGroupId: $claGroupId}
     + (if $returnUrl == "" then {} else {returnUrl: $returnUrl} end)
     + (if $lfUsername == "" then {} else {lfUsername: $lfUsername} end)
     + (if $email == "" then {} else {email: $email} end)
     + (if $githubId == "" then {} else {githubId: ($githubId | tonumber)} end)
     + (if $githubUsername == "" then {} else {githubUsername: $githubUsername} end)
     + (if $gitlabId == "" then {} else {gitlabId: ($gitlabId | tonumber)} end)
     + (if $gitlabUsername == "" then {} else {gitlabUsername: $gitlabUsername} end)
     + (if $gerritUsername == "" then {} else {gerritUsername: $gerritUsername} end)')"
  URL="${API_URL}/v4/self-serve/prepare-sign"
  if [ -n "$SIGN_ICLA" ]
  then
    if [ -z "$USER_ID" ]
    then
      echo "$0: USER_ID not set - pass the EasyCLA user UUID returned by prepare-sign"
      rm -f "$body"
      exit 6
    fi
    payload="$(jq -nc \
      --arg projectId "$CLA_GROUP_ID" \
      --arg userId "$USER_ID" \
      --arg returnUrlType "${RETURN_URL_TYPE:-Github}" \
      --arg returnUrl "$RETURN_URL" \
      '{project_id: $projectId, user_id: $userId, return_url_type: $returnUrlType}
       + (if $returnUrl == "" then {} else {return_url: $returnUrl} end)')"
    URL="${API_URL}/v4/request-individual-signature"
  fi
  [ -n "$DEBUG" ] && echo "curl -sS -XPOST ${auth[0]} '<redacted>' -H 'Content-Type: application/json' -d '${payload}' '${URL}'"
  timing="$(curl -sS -XPOST "${auth[@]}" -H "Content-Type: application/json" -d "$payload" -w '%{http_code} %{time_total}' -o "$body" "$URL")"
fi

if command -v jq >/dev/null 2>&1
then
  jq -r '.' < "$body" 2>/dev/null || cat "$body"
else
  cat "$body"
fi
echo
echo "HTTP ${timing% *} in ${timing#* }s"
rm -f "$body"
