#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# Audits companies-table reachability (#2054): classifies every cla-{STAGE}-companies row by
# whether its company_external_id (SFID) is missing, resolves in the org-service, or no longer
# does, and counts active (signed+approved) CCLAs/ECLAs per row so data fixes can be prioritized.
# Tiers: MISSING_SFID | SFID_OK | SFID_DANGLING_OR_DELETED | UNKNOWN (org-service 404s both a
# never-existed SFID and a soft-deleted SF Account, so those collapse into one tier).
# TSV rows go to stdout ('#'-prefixed summary at the end), progress goes to stderr.
# STAGE: dev (default) | test | staging | prod. AWS_REGION defaults to us-east-1.
# TOKEN: optional bearer token for the org-service SFID liveness check - without it every row
# that has an SFID is tiered UNKNOWN (only MISSING_SFID is authoritative in that mode).
# SLEEP_S: optional seconds to sleep after each org-service call.
# Usage: [STAGE=dev] [TOKEN=...] ./utils/audit_company_reachability.sh > companies_audit.tsv

if [ -z "$STAGE" ]
then
  STAGE=dev
fi
if [ -z "$AWS_REGION" ]
then
  export AWS_REGION=us-east-1
fi
case "$STAGE" in
  prod) GW="https://api-gw.platform.linuxfoundation.org" ;;
  staging) GW="https://api-gw.staging.platform.linuxfoundation.org" ;;
  test) GW="https://api-gw.test.platform.linuxfoundation.org" ;;
  dev) GW="https://api-gw.dev.platform.linuxfoundation.org" ;;
  *) echo "$0: unknown STAGE '$STAGE'" >&2; exit 2 ;;
esac

# Sums Count across query pages; the GSI queries mirror the production lookups:
# CCLA = GetCompanySignatures (reference-signature-index), ECLA = the
# signature-user-ccla-company-index partition (only ECLA rows carry that attribute).
count_query () {
  local index="$1" key_expr="$2" filter="$3" values="$4"
  local total=0 lek="" page
  while :
  do
    local args=(--profile "lfproduct-${STAGE}" dynamodb query --table-name "cla-${STAGE}-signatures" \
      --index-name "$index" --key-condition-expression "$key_expr" \
      --filter-expression "$filter" --expression-attribute-values "$values" --select COUNT)
    [ -n "$lek" ] && args+=(--exclusive-start-key "$lek")
    if ! page="$(aws "${args[@]}")"
    then
      echo "$0: signatures count query failed (index ${index}) - reporting -1 for this row" >&2
      echo "-1"
      return
    fi
    total=$((total + $(echo "$page" | jq -r '.Count // 0')))
    lek="$(echo "$page" | jq -c '.LastEvaluatedKey // empty')"
    [ -z "$lek" ] && break
  done
  echo "$total"
}

classify () {
  local sfid="$1"
  if [ -z "$sfid" ]
  then
    echo "MISSING_SFID"
    return
  fi
  if [ -z "$TOKEN" ]
  then
    echo "UNKNOWN"
    return
  fi
  local code
  code="$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer ${TOKEN}" "${GW}/organization-service/v1/orgs/${sfid}")"
  [ -n "$SLEEP_S" ] && sleep "$SLEEP_S"
  case "$code" in
    200) echo "SFID_OK" ;;
    404) echo "SFID_DANGLING_OR_DELETED" ;;
    *) echo "$0: org-service returned HTTP ${code} for SFID ${sfid}" >&2; echo "UNKNOWN" ;;
  esac
}

echo -e "company_id\tcompany_name\tsigning_entity_name\tcompany_external_id\tsfid_status\tactive_ccla_count\tactive_ecla_count"

total=0
missing=0
ok=0
dangling=0
unknown=0
missing_with_active_ccla=0
count_failures=0
lek=""
while :
do
  scan_args=(--profile "lfproduct-${STAGE}" dynamodb scan --table-name "cla-${STAGE}-companies" \
    --projection-expression "company_id,company_name,signing_entity_name,company_external_id")
  [ -n "$lek" ] && scan_args+=(--exclusive-start-key "$lek")
  if ! scan_page="$(aws "${scan_args[@]}")"
  then
    echo "$0: companies scan failed" >&2
    exit 3
  fi
  while IFS= read -r item
  do
    [ -z "$item" ] && continue
    company_id="$(echo "$item" | jq -r '.company_id.S // ""')"
    company_name="$(echo "$item" | jq -r '.company_name.S // ""')"
    signing_entity_name="$(echo "$item" | jq -r '.signing_entity_name.S // ""')"
    sfid="$(echo "$item" | jq -r '.company_external_id.S // ""')"

    status="$(classify "$sfid")"
    ccla_count="$(count_query reference-signature-index "signature_reference_id = :cid" \
      "signature_type = :t AND signature_approved = :b AND signature_signed = :b" \
      "{\":cid\":{\"S\":\"${company_id}\"},\":t\":{\"S\":\"ccla\"},\":b\":{\"BOOL\":true}}")"
    ecla_count="$(count_query signature-user-ccla-company-index "signature_user_ccla_company_id = :cid" \
      "signature_approved = :b AND signature_signed = :b" \
      "{\":cid\":{\"S\":\"${company_id}\"},\":b\":{\"BOOL\":true}}")"

    echo -e "${company_id}\t${company_name}\t${signing_entity_name}\t${sfid}\t${status}\t${ccla_count}\t${ecla_count}"

    { [ "$ccla_count" = "-1" ] || [ "$ecla_count" = "-1" ]; } && count_failures=$((count_failures + 1))
    total=$((total + 1))
    case "$status" in
      MISSING_SFID)
        missing=$((missing + 1))
        [ "$ccla_count" -gt 0 ] 2>/dev/null && missing_with_active_ccla=$((missing_with_active_ccla + 1))
        ;;
      SFID_OK) ok=$((ok + 1)) ;;
      SFID_DANGLING_OR_DELETED) dangling=$((dangling + 1)) ;;
      *) unknown=$((unknown + 1)) ;;
    esac
    if [ $((total % 100)) -eq 0 ]
    then
      echo "$0: processed ${total} companies..." >&2
    fi
  done < <(echo "$scan_page" | jq -c '.Items[]')
  lek="$(echo "$scan_page" | jq -c '.LastEvaluatedKey // empty')"
  [ -z "$lek" ] && break
done

echo "#"
echo "# total companies scanned: ${total}"
echo "# MISSING_SFID: ${missing} (of which with active CCLA: ${missing_with_active_ccla})"
echo "# SFID_OK: ${ok}"
if [ -z "$TOKEN" ]
then
  echo "# SFID_DANGLING_OR_DELETED: ${dangling} (TOKEN unset - liveness not checked)"
  echo "# UNKNOWN: ${unknown} (TOKEN unset - every row with an SFID lands here)"
else
  echo "# SFID_DANGLING_OR_DELETED: ${dangling}"
  echo "# UNKNOWN: ${unknown}"
fi
if [ "$count_failures" -gt 0 ]
then
  echo "# rows with FAILED signature count queries (-1 in TSV, excluded from the active-CCLA tally - re-run these): ${count_failures}"
fi
echo "# total unreachable (authoritative = MISSING_SFID + SFID_DANGLING_OR_DELETED): $((missing + dangling))"
