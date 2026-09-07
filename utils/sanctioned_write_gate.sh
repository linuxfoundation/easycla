#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# Probes the sanctioned-company write gate (lfx-self-serve #2153): calls every gated v4 write op against a
# company whose is_sanctioned flag is set and PASSes only on HTTP 403 with a "company_sanctioned" body.
# Exits non-zero if any probed op responds otherwise. Probe payloads use bogus target identifiers
# (nonexistent emails/LFIDs, a bogus requestID) so even a broken gate mutates nothing meaningful -
# except eclaAutoCreate, which has no bogus placeholder (the payload IS the real CCLA's flag; bogus
# company/CLA-group IDs 404 before the gate), so it only runs with ECLA_AUTO_CREATE_OK=1: if the gate
# is broken it persists auto_create_ecla=false on the real COMPANY_ID/CLA_GROUP_ID CCLA (re-enable afterwards).
# updateApprovalList also targets the real CCLA: a broken gate rewrites its email approval-list column
# unchanged and adds an inactive approval-history row for the bogus email (junk only, no approval-state change).
# Gated ops probed: updateApprovalList, createCLAManager, deleteCLAManager,
# createCLAManagerDesignee, createCLAManagerDesigneeByGroup, createCLAManagerRequest,
# approveCLAManagerRequest, denyCLAManagerRequest, and optionally eclaAutoCreate (needs
# ECLA_AUTO_CREATE_OK=1, see above), inviteCompanyAdmin (needs USER_ID, a real EasyCLA user UUID -
# the handler loads the user before the gate) and invalidateECLA (needs ECLA_SIG_ID - use an
# ALREADY-INVALIDATED ECLA of the sanctioned company so a broken gate yields 409, not a mutation).
# COMPANY_ID: internal company UUID with is_sanctioned=true (default: dev CNCF). COMPANY_SFID: its Salesforce ID.
# PROJECT_SFID / CLA_GROUP_ID: any valid pairing (the gate fires before CLA-group mapping is consulted,
# but the CLA group must have SF project mappings for the by-group designee op).
# OPS: optional space-separated subset of op names to probe.
# Caller scope: updateApprovalList, create/deleteCLAManager and approve/denyCLAManagerRequest check the
# project|organization pair scope (admin disallowed) BEFORE the gate, and createCLAManagerRequest checks the
# organization scope - a 403 "does not have access" body means your caller lacks scope, not that the gate works.
# eclaAutoCreate additionally requires the caller to be in the CCLA signature ACL.
# TOKEN: bearer access token (env, or ./sanctioned_write_gate.token.secret / ./auth0.token.secret). Get one with ~/get_oauth_token.sh (dev) or ~/get_oauth_token_prod.sh (prod).
# STAGE: dev (default) | test | staging | prod - selects the api-gw host.
# Local mode (against a standalone backend, bypassing the gateway): set PRINCIPAL to the token username
# (or pass a raw base64 X_ACL); the crafted X-ACL carries isAdmin (ADMIN=true|false, default true), the
# project|organization pair scope and the organization scope, so all pre-gate authz checks pass. Defaults API_URL to http://localhost:8080.
# Examples:
#   ./utils/sanctioned_write_gate.sh
#   OPS="createCLAManagerDesignee createCLAManagerRequest" ./utils/sanctioned_write_gate.sh
#   PRINCIPAL=lgryglicki ./utils/sanctioned_write_gate.sh
#   ECLA_SIG_ID=<already-invalidated-ecla-uuid> USER_ID=<easycla-user-uuid> ECLA_AUTO_CREATE_OK=1 ./utils/sanctioned_write_gate.sh

[ -z "$COMPANY_ID" ] && COMPANY_ID='0ca30016-6457-466c-bc41-a09560c1f9bf'
[ -z "$COMPANY_SFID" ] && COMPANY_SFID='0014100000Te0yqAAB'
[ -z "$PROJECT_SFID" ] && PROJECT_SFID='a09P000000DsCE5IAN'
[ -z "$CLA_GROUP_ID" ] && CLA_GROUP_ID='d8cead54-92b7-48c5-a2c8-b1e295e8f7f1'
PROBE_EMAIL='sanctions-gate-probe@example.com'
PROBE_LFID='sanctions-gate-probe-nouser'
PROBE_REQUEST_ID='00000000-0000-4000-8000-000000000000'

if [ -n "$PRINCIPAL" ] && [ -z "$X_ACL" ]
then
  admin=true
  [ "$ADMIN" = "false" ] && admin=false
  scopes="\"scopes\":[{\"type\":\"project|organization\",\"id\":\"${PROJECT_SFID}|${COMPANY_SFID}\"},{\"type\":\"organization\",\"id\":\"${COMPANY_SFID}\"}]"
  X_ACL="$(printf '{"user_name":"%s","email":"%s","isAdmin":%s,"allowed":true,%s}' "$PRINCIPAL" "${PRINCIPAL_EMAIL:-$PRINCIPAL}" "$admin" "$scopes" | base64 | tr -d '\n')"
fi

if [ -n "$X_ACL" ]
then
  auth=(-H "X-ACL: ${X_ACL}" -H "Authorization: Bearer ${TOKEN:-local}" -H "X-USERNAME: ${PRINCIPAL:-local}" -H "X-EMAIL: ${PRINCIPAL_EMAIL:-${PRINCIPAL:-local}}")
  [ -z "$API_URL" ] && API_URL="http://localhost:${PORT:-8080}"
else
  if [ -z "$TOKEN" ]
  then
    [ -f ./sanctioned_write_gate.token.secret ] && TOKEN="$(cat ./sanctioned_write_gate.token.secret)"
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

[ -z "$OPS" ] && OPS='updateApprovalList eclaAutoCreate invalidateECLA createCLAManager deleteCLAManager createCLAManagerDesignee createCLAManagerDesigneeByGroup inviteCompanyAdmin createCLAManagerRequest approveCLAManagerRequest denyCLAManagerRequest'

pass=0
fail=0
skip=0
failed_ops=''
body="$(mktemp)"
trap 'rm -f "$body"' EXIT

probe() {
  local op="$1" method="$2" url="$3" payload="$4"
  local data=()
  [ -n "$payload" ] && data=(-H "Content-Type: application/json" -d "$payload")
  if [ -n "$DEBUG" ]
  then
    echo "curl -sS -X${method} '${auth[0]}: <redacted>' ${payload:+-d '${payload}' }'${API_URL}/v4${url}'"
  fi
  local timing
  timing="$(curl -sS -X "$method" "${auth[@]}" "${data[@]}" -w '%{http_code} %{time_total}' -o "$body" "${API_URL}/v4${url}")"
  local code="${timing% *}"
  if [ "$code" = "403" ] && grep -q 'company_sanctioned' "$body"
  then
    echo "PASS ${op}: HTTP 403 company_sanctioned in ${timing#* }s"
    pass=$((pass+1))
  else
    echo "FAIL ${op}: HTTP ${code} in ${timing#* }s"
    cat "$body"
    echo
    fail=$((fail+1))
    failed_ops="${failed_ops} ${op}"
  fi
}

for op in $OPS
do
  case "$op" in
    updateApprovalList)
      probe "$op" PUT "/signatures/project/${PROJECT_SFID}/company/${COMPANY_ID}/clagroup/${CLA_GROUP_ID}/approval-list" "{\"RemoveEmailApprovalList\":[\"${PROBE_EMAIL}\"]}"
      ;;
    eclaAutoCreate)
      if [ "$ECLA_AUTO_CREATE_OK" != "1" ]
      then
        echo "SKIP ${op}: set ECLA_AUTO_CREATE_OK=1 to probe it - no bogus placeholder exists for this op, so a broken gate persists auto_create_ecla=false on the real CCLA"
        skip=$((skip+1))
      else
        probe "$op" PUT "/signatures/company/${COMPANY_ID}/clagroup/${CLA_GROUP_ID}/ecla-auto-create" '{"auto_create_ecla":false}'
      fi
      ;;
    invalidateECLA)
      if [ -z "$ECLA_SIG_ID" ]
      then
        echo "SKIP ${op}: set ECLA_SIG_ID to an already-invalidated ECLA signature UUID of the sanctioned company to probe it"
        skip=$((skip+1))
      else
        probe "$op" PUT "/cla-group/${CLA_GROUP_ID}/ecla/${ECLA_SIG_ID}/invalidate" '{"reason":"other","note":"sanctions gate probe"}'
      fi
      ;;
    createCLAManager)
      probe "$op" POST "/company/${COMPANY_ID}/project/${PROJECT_SFID}/cla-manager" "{\"firstName\":\"Sanctions\",\"lastName\":\"GateProbe\",\"userEmail\":\"${PROBE_EMAIL}\"}"
      ;;
    deleteCLAManager)
      probe "$op" DELETE "/company/${COMPANY_ID}/project/${PROJECT_SFID}/cla-manager/${PROBE_LFID}" ''
      ;;
    createCLAManagerDesignee)
      probe "$op" POST "/company/${COMPANY_ID}/project/${PROJECT_SFID}/cla-manager-designee" "{\"userEmail\":\"${PROBE_EMAIL}\"}"
      ;;
    createCLAManagerDesigneeByGroup)
      probe "$op" POST "/company/${COMPANY_ID}/claGroup/${CLA_GROUP_ID}/cla-manager-designee" "{\"userEmail\":\"${PROBE_EMAIL}\"}"
      ;;
    inviteCompanyAdmin)
      if [ -z "$USER_ID" ]
      then
        echo "SKIP ${op}: set USER_ID to a real EasyCLA user UUID (the handler loads the user before the gate) to probe it"
        skip=$((skip+1))
      else
        probe "$op" POST "/user/${USER_ID}/invite-company-admin" "{\"claGroupID\":\"${CLA_GROUP_ID}\",\"companyID\":\"${COMPANY_ID}\",\"contactAdmin\":false,\"name\":\"Sanctions Gate Probe\",\"userEmail\":\"${PROBE_EMAIL}\"}"
      fi
      ;;
    createCLAManagerRequest)
      probe "$op" POST "/company/${COMPANY_ID}/project/${PROJECT_SFID}/cla-manager/requests" "{\"contactAdmin\":false,\"fullName\":\"Sanctions Gate Probe\",\"userEmail\":\"${PROBE_EMAIL}\"}"
      ;;
    approveCLAManagerRequest)
      probe "$op" PUT "/company/${COMPANY_ID}/project/${PROJECT_SFID}/cla-manager/requests/${PROBE_REQUEST_ID}/approve" ''
      ;;
    denyCLAManagerRequest)
      probe "$op" PUT "/company/${COMPANY_ID}/project/${PROJECT_SFID}/cla-manager/requests/${PROBE_REQUEST_ID}/deny" ''
      ;;
    *) echo "$0: unknown op '$op'"; exit 2 ;;
  esac
done

echo "${pass} passed, ${fail} failed, ${skip} skipped"
if [ "$fail" -gt 0 ]
then
  echo "failed ops:${failed_ops}"
  exit 4
fi
