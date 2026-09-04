#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# Calls the M3 managers & acknowledgments write ops (lfx-self-serve #2151) through lfx-gateway and reports the HTTP status and total time.
# OP: list (default) | get | approve | deny | invalidate-ecla.
#   list:            GET /v4/company/{COMPANY_ID}/project/{PROJECT_SFID}/cla-manager/requests
#   get:             GET /v4/company/{COMPANY_ID}/project/{PROJECT_SFID}/cla-manager/requests/{REQUEST_ID}
#   approve:         PUT /v4/company/{COMPANY_ID}/project/{PROJECT_SFID}/cla-manager/requests/{REQUEST_ID}/approve
#   deny:            PUT /v4/company/{COMPANY_ID}/project/{PROJECT_SFID}/cla-manager/requests/{REQUEST_ID}/deny
#   invalidate-ecla: PUT /v4/cla-group/{CLA_GROUP_ID}/ecla/{SIGNATURE_ID}/invalidate
# WARNING: a 200 on approve/deny updates the request status (approve also adds the requester to the CCLA signature ACL) and sends emails; a 200 on invalidate-ecla sets the ECLA signature_approved=false and emails the employee.
# COMPANY_ID (or 1st arg, internal UUID) and PROJECT_SFID (or 2nd arg): required for list/get/approve/deny; REQUEST_ID (or 3rd arg): required for get/approve/deny.
# CLA_GROUP_ID (or 1st arg) and SIGNATURE_ID (or 2nd arg): required for invalidate-ecla; REASON / NOTE: optional invalidation body fields.
# TOKEN: bearer access token (env, or ./cla_manager_requests_ops.token.secret / ./auth0.token.secret). Get one with ~/get_oauth_token.sh (dev) or ~/get_oauth_token_prod.sh (prod).
# STAGE: dev (default) | test | staging | prod - selects the api-gw host.
# Local mode (against a standalone backend, bypassing the gateway): set PRINCIPAL to the token username and ADMIN=true|false (or pass a raw base64 X_ACL); SCOPE="projectSFID|companySFID" adds a project|organization scope (required by these endpoints - admin is disallowed). Defaults API_URL to http://localhost:8080.
# Examples:
#   ./utils/cla_manager_requests_ops.sh f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc a09P000000DsCE5IAN
#   OP=get REQUEST_ID=<uuid> ./utils/cla_manager_requests_ops.sh f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc a09P000000DsCE5IAN
#   OP=invalidate-ecla REASON=offboarded ./utils/cla_manager_requests_ops.sh 01af041c-fa69-4052-a23c-fb8c1d3bef24 <signature-uuid>
#   PRINCIPAL=lgryglicki SCOPE='a09P000000DsCE5IAN|00117000015vpjXAAQ' ./utils/cla_manager_requests_ops.sh f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc a09P000000DsCE5IAN

[ -z "$OP" ] && OP=list
case "$OP" in
  list|get|approve|deny)
    [ -z "$COMPANY_ID" ] && COMPANY_ID="$1"
    [ -z "$PROJECT_SFID" ] && PROJECT_SFID="$2"
    [ -z "$REQUEST_ID" ] && REQUEST_ID="$3"
    if [ -z "$COMPANY_ID" ] || [ -z "$PROJECT_SFID" ]
    then
      echo "$0: COMPANY_ID (or 1st arg) and PROJECT_SFID (or 2nd arg) are required for OP=$OP"
      exit 3
    fi
    if [ "$OP" != "list" ] && [ -z "$REQUEST_ID" ]
    then
      echo "$0: REQUEST_ID (or 3rd arg) is required for OP=$OP"
      exit 3
    fi
    ;;
  invalidate-ecla)
    [ -z "$CLA_GROUP_ID" ] && CLA_GROUP_ID="$1"
    [ -z "$SIGNATURE_ID" ] && SIGNATURE_ID="$2"
    if [ -z "$CLA_GROUP_ID" ] || [ -z "$SIGNATURE_ID" ]
    then
      echo "$0: CLA_GROUP_ID (or 1st arg) and SIGNATURE_ID (or 2nd arg) are required for OP=$OP"
      exit 3
    fi
    ;;
  *) echo "$0: unknown OP '$OP' (list|get|approve|deny|invalidate-ecla)"; exit 2 ;;
esac

if [ -n "$PRINCIPAL" ] && [ -z "$X_ACL" ]
then
  admin=false
  [ "$ADMIN" = "true" ] && admin=true
  scopes=""
  [ -n "$SCOPE" ] && scopes=",\"scopes\":[{\"type\":\"project|organization\",\"id\":\"${SCOPE}\"}]"
  X_ACL="$(printf '{"user_name":"%s","email":"%s","isAdmin":%s,"allowed":true%s}' "$PRINCIPAL" "${PRINCIPAL_EMAIL:-$PRINCIPAL}" "$admin" "$scopes" | base64 | tr -d '\n')"
fi

if [ -n "$X_ACL" ]
then
  auth=(-H "X-ACL: ${X_ACL}" -H "Authorization: Bearer ${TOKEN:-local}" -H "X-USERNAME: ${PRINCIPAL:-local}" -H "X-EMAIL: ${PRINCIPAL_EMAIL:-${PRINCIPAL:-local}}")
  [ -z "$API_URL" ] && API_URL="http://localhost:${PORT:-8080}"
else
  if [ -z "$TOKEN" ]
  then
    [ -f ./cla_manager_requests_ops.token.secret ] && TOKEN="$(cat ./cla_manager_requests_ops.token.secret)"
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

method=GET
payload=""
case "$OP" in
  list) URL="${API_URL}/v4/company/${COMPANY_ID}/project/${PROJECT_SFID}/cla-manager/requests" ;;
  get) URL="${API_URL}/v4/company/${COMPANY_ID}/project/${PROJECT_SFID}/cla-manager/requests/${REQUEST_ID}" ;;
  approve) method=PUT; URL="${API_URL}/v4/company/${COMPANY_ID}/project/${PROJECT_SFID}/cla-manager/requests/${REQUEST_ID}/approve" ;;
  deny) method=PUT; URL="${API_URL}/v4/company/${COMPANY_ID}/project/${PROJECT_SFID}/cla-manager/requests/${REQUEST_ID}/deny" ;;
  invalidate-ecla)
    method=PUT
    URL="${API_URL}/v4/cla-group/${CLA_GROUP_ID}/ecla/${SIGNATURE_ID}/invalidate"
    payload="{"
    [ -n "$REASON" ] && payload="${payload}\"reason\":\"${REASON}\""
    if [ -n "$NOTE" ]
    then
      [ "$payload" != "{" ] && payload="${payload},"
      payload="${payload}\"note\":\"${NOTE}\""
    fi
    payload="${payload}}"
    ;;
esac

data=()
[ -n "$payload" ] && data=(-H "Content-Type: application/json" -d "$payload")

if [ -n "$DEBUG" ]
then
  echo "curl -sS -X${method} '${auth[0]}: <redacted>' ${payload:+-d '${payload}' }'${URL}'"
fi

body="$(mktemp)"
timing="$(curl -sS -X "$method" "${auth[@]}" "${data[@]}" -w '%{http_code} %{time_total}' -o "$body" "$URL")"
if command -v jq >/dev/null 2>&1
then
  jq -r '.' < "$body" 2>/dev/null || cat "$body"
else
  cat "$body"
fi
echo
echo "HTTP ${timing% *} in ${timing#* }s"
rm -f "$body"
