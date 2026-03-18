#!/bin/bash
#
# Note: data returned from "scan.sh" has escape characters, like '\\`, use `\` when putting that data back to DynamoDB (via this script) to avoid double escaping.
#
# MODE=mode ./utils/skip_cla_entry.sh sun-test-org '*' 'patterns'
# put-item        Overwrites/adds the entire `skip_cla` entry.
# add-key         Adds or updates a key/value inside the skip_cla map (preserves other keys)
# add-key-item    Adds one pattern item into an existing skip_cla key array (idempotent)
# delete-key      Removes a key from the skip_cla map
# delete-item     Deletes the entire `skip_cla` entry.
# delete-key-item Removes one pattern item from an existing skip_cla key array (idempotent)
#
# MODE=add-key ./utils/skip_cla_entry.sh sun-test-org 'repo1' 're:vee?rendra;*;*'
# MODE=add-key ./utils/skip_cla_entry.sh 'sun-test-org' 'repo1' 'lukaszgryglicki;re:gryglicki'
# MODE=add-key ./utils/skip_cla_entry.sh 'sun-test-org' 're:(?i)^repo[0-9]+$' '[re:(?i)^l(ukasz)?gryglicki$;re:(?i)^l(ukasz)?gryglicki@;*||copilot-swe-agent[bot]]'
# ./utils/scan.sh github-orgs organization_name sun-test-org
# STAGE=dev DTFROM='1 hour ago' DTTO='1 second ago' ./utils/search_aws_log_group.sh 'cla-backend-dev-githubactivity' 'skip_cla'
# MODE=delete-key ./utils/skip_cla_entry.sh 'sun-test-org' 're:(?i)^repo[0-9]+$'
# STAGE=dev MODE=add-key DEBUG=1 ./utils/skip_cla_entry.sh 'sun-test-org' 'repo1' 'thakurveerendras;;*'
# STAGE=dev MODE=add-key ./utils/skip_cla_entry.sh 'open-telemetry' '*' 'Copilot;re:^\d+\+Copilot@users\.noreply\.github\.com$;copilot-swe-agent[bot]'
# STAGE=dev MODE=add-key ./utils/skip_cla_entry.sh 'openfga' 'vscode-ext' 'Copilot;re:^\d+\+Copilot@users\.noreply\.github\.com$;copilot-swe-agent[bot]'
# STAGE=prod MODE=add-key DEBUG=1 ./utils/skip_cla_entry.sh 'open-telemetry' 'opentelemetry-rust' '*;re:^\d+\+Copilot@users\.noreply\.github\.com$;copilot-swe-agent[bot]'
# STAGE=prod MODE=add-key ./utils/skip_cla_entry.sh 'openfga' 'vscode-ext' '[Copilot;re:^\d+\+Copilot@users\.noreply\.github\.com$;copilot-swe-agent[bot]||;re:^\d+\+Copilot@users\.noreply\.github\.com$;copilot-swe-agent[bot]]'
# STAGE=prod MODE=add-key-item ./utils/skip_cla_entry.sh 'openfga' '*' 're:(?i)^copilot$;re:(?i)^\d+\+copilot@users\.noreply\.github\.com$;*'
# STAGE=prod MODE=delete-key-item ./utils/skip_cla_entry.sh 'openfga' '*' 're:(?i)^copilot$;re:(?i)^\d+\+copilot@users\.noreply\.github\.com$;*'

if [ -z "$MODE" ]
then
  echo "$0: MODE must be set, valid values are: put-item, add-key, delete-key, delete-item, add-key-item, delete-key-item"
  exit 1
fi

if [ -z "$STAGE" ]; then
  STAGE='dev'
fi
if [ -z "$REGION" ]; then
  REGION='us-east-1'
fi

aws_key_json() {
  jq -cn --arg org "$1" '{"organization_name": {"S": $org}}'
}

aws_attr_names_json() {
  jq -cn --arg repo "$1" '{"#repo": $repo}'
}

aws_attr_value_string_json() {
  jq -cn --arg val "$1" '{":val": {"S": $val}}'
}

get_skip_cla_key_value() {
  local org="$1"
  local repo="$2"
  aws --profile "lfproduct-${STAGE}" --region "${REGION}" dynamodb get-item \
    --table-name "cla-${STAGE}-github-orgs" \
    --key "$(aws_key_json "$org")" \
    --projection-expression 'skip_cla' \
  | jq -r --arg repo "$repo" '.Item.skip_cla.M[$repo].S // empty'
}

modify_skip_cla_array_value() {
  local current="$1"
  local action="$2"
  local item="$3"
  CURRENT="$current" ACTION="$action" ITEM="$item" python3 - <<'PY'
import os

current = os.environ.get("CURRENT", "")
action = os.environ["ACTION"]
item = os.environ["ITEM"]

if current.startswith("[") and current.endswith("]"):
    inner = current[1:-1]
    items = [p.strip() for p in inner.split("||")] if inner else []
else:
    items = [current.strip()] if current.strip() else []

items = [p for p in items if p != ""]
changed = False

if action == "add":
    if item not in items:
        items.append(item)
        changed = True
elif action == "delete":
    new_items = [p for p in items if p != item]
    changed = (new_items != items)
    items = new_items
else:
    raise SystemExit(f"unsupported action: {action}")

if not items:
    new_value = ""
else:
    new_value = "[" + "||".join(items) + "]"

print("changed=true" if changed else "changed=false")
print("delete_key=true" if changed and new_value == "" else "delete_key=false")
print(new_value)
PY
}

case "$MODE" in
  put-item)
    if ( [ -z "${1}" ] || [ -z "${2}" ] || [ -z "${3}" ] ); then
      echo "Usage: $0 <organization_name> <repo or re:repo-regexp or *> <patterns or array-of-patterns>"
      exit 1
    fi
    repo=$(echo "${2}" | sed 's/\\/\\\\/g')
    pat=$(echo "${3}" | sed 's/\\/\\\\/g')
    CMD="aws --profile \"lfproduct-${STAGE}\" --region \"${REGION}\" dynamodb update-item \
      --table-name \"cla-${STAGE}-github-orgs\" \
      --key '{\"organization_name\": {\"S\": \"${1}\"}}' \
      --update-expression 'SET skip_cla = :val' \
      --expression-attribute-values '{\":val\": {\"M\": {\"${repo}\":{\"S\":\"${pat}\"}}}}'"
    ;;
  add-key)
    if ( [ -z "${1}" ] || [ -z "${2}" ] || [ -z "${3}" ] ); then
      echo "Usage: $0 <organization_name> <repo or re:repo-regexp or *> <patterns or array-of-patterns>"
      exit 1
    fi
    repo=$(echo "${2}" | sed 's/\\/\\\\/g')
    pat=$(echo "${3}" | sed 's/\\/\\\\/g')
    CMD="aws --profile \"lfproduct-${STAGE}\" --region \"${REGION}\" dynamodb update-item \
      --table-name \"cla-${STAGE}-github-orgs\" \
      --key '{\"organization_name\": {\"S\": \"${1}\"}}' \
      --update-expression 'SET skip_cla.#repo = :val' \
      --expression-attribute-names '{\"#repo\": \"${repo}\"}' \
      --expression-attribute-values '{\":val\": {\"S\": \"${pat}\"}}'"
    ;;
  delete-key)
    if ( [ -z "${1}" ] || [ -z "${2}" ] ); then
      echo "Usage: $0 <organization_name> <repo or re:repo-regexp or *>"
      exit 1
    fi
    repo=$(echo "${2}" | sed 's/\\/\\\\/g')
    CMD="aws --profile \"lfproduct-${STAGE}\" --region \"${REGION}\" dynamodb update-item \
      --table-name \"cla-${STAGE}-github-orgs\" \
      --key '{\"organization_name\": {\"S\": \"${1}\"}}' \
      --update-expression 'REMOVE skip_cla.#repo' \
      --expression-attribute-names '{\"#repo\": \"${repo}\"}'"
    ;;
  delete-item)
    if [ -z "${1}" ]; then
      echo "Usage: $0 <organization_name>"
      exit 1
    fi
    CMD="aws --profile \"lfproduct-${STAGE}\" --region \"${REGION}\" dynamodb update-item \
      --table-name \"cla-${STAGE}-github-orgs\" \
      --key '{\"organization_name\": {\"S\": \"${1}\"}}' \
      --update-expression 'REMOVE skip_cla'"
    ;;
  add-key-item|delete-key-item)
    if ( [ -z "${1}" ] || [ -z "${2}" ] || [ -z "${3}" ] ); then
      echo "Usage: $0 <organization_name> <repo or re:repo-regexp or *> <pattern-item>"
      exit 1
    fi

    org_name="${1}"
    repo_key="${2}"
    item_value="${3}"
    current_value="$(get_skip_cla_key_value "$org_name" "$repo_key")"

    if [ "$MODE" = "add-key-item" ]; then
      action="add"
    else
      action="delete"
    fi

    mapfile -t result_lines < <(modify_skip_cla_array_value "$current_value" "$action" "$item_value")
    changed="${result_lines[0]#changed=}"
    delete_key="${result_lines[1]#delete_key=}"
    new_value="${result_lines[2]}"

    if [ "$changed" != "true" ]; then
      if [ ! -z "$DEBUG" ]; then
        echo "No changes needed for organization='${org_name}', repo='${repo_key}', mode='${MODE}'"
      fi
      exit 0
    fi

    if [ "$delete_key" = "true" ]; then
      if [ ! -z "$DEBUG" ]; then
        echo "aws --profile \"lfproduct-${STAGE}\" --region \"${REGION}\" dynamodb update-item --table-name \"cla-${STAGE}-github-orgs\" --key \"$(aws_key_json "$org_name")\" --update-expression 'REMOVE skip_cla.#repo' --expression-attribute-names \"$(aws_attr_names_json "$repo_key")\""
      fi
      aws --profile "lfproduct-${STAGE}" --region "${REGION}" dynamodb update-item \
        --table-name "cla-${STAGE}-github-orgs" \
        --key "$(aws_key_json "$org_name")" \
        --update-expression 'REMOVE skip_cla.#repo' \
        --expression-attribute-names "$(aws_attr_names_json "$repo_key")"
      exit $?
    fi

    if [ ! -z "$DEBUG" ]; then
      echo "aws --profile \"lfproduct-${STAGE}\" --region \"${REGION}\" dynamodb update-item --table-name \"cla-${STAGE}-github-orgs\" --key \"$(aws_key_json "$org_name")\" --update-expression 'SET skip_cla.#repo = :val' --expression-attribute-names \"$(aws_attr_names_json "$repo_key")\" --expression-attribute-values \"$(aws_attr_value_string_json "$new_value")\""
    fi
    aws --profile "lfproduct-${STAGE}" --region "${REGION}" dynamodb update-item \
      --table-name "cla-${STAGE}-github-orgs" \
      --key "$(aws_key_json "$org_name")" \
      --update-expression 'SET skip_cla.#repo = :val' \
      --expression-attribute-names "$(aws_attr_names_json "$repo_key")" \
      --expression-attribute-values "$(aws_attr_value_string_json "$new_value")"
    exit $?
    ;;
  *)
    echo "$0: Unknown MODE: $MODE"
    echo "Valid values are: put-item, add-key, delete-key, delete-item, add-key-item, delete-key-item"
    exit 1
    ;;
esac

if [ ! -z "$DEBUG" ]
then
  echo "$CMD"
fi

eval $CMD
