#!/bin/bash
# MODE=mode ./utils/skip_cla_entry.sh sun-test-org '*' 'copilot-swe-agent[bot]' '*'
# put-item    Overwrites the entire item (skip_cla and all other attributes if needed)
# add-key     Adds or updates a key/value inside the skip_cla map (preserves other keys)
# delete-key  Removes a key from the skip_cla map
# delete-item Deletes the entire DynamoDB item (removes the whole row)
#
# MODE=add-key ./utils/skip_cla_entry.sh sun-test-org 'repo1' 're:vee?rendra' '*'
# ./utils/scan.sh github-orgs organization_name sun-test-org

if [ -z "$MODE" ]
then
  echo "$0: MODE must be set, valid values are: put-item, add-key, delete-key, delete-item"
  exit 1
fi

if [ -z "$STAGE" ]; then
  STAGE=dev
fi
if [ -z "$REGION" ]; then
  REGION=us-east-1
fi

case "$MODE" in
  put-item)
    if ( [ -z "${1}" ] || [ -z "${2}" ] || [ -z "${3}" ] || [ -z "${4}" ] ); then
      echo "Usage: $0 <organization_name> <repo or *> <bot username> <email regexp>"
      exit 1
    fi
    aws --profile "lfproduct-${STAGE}" --region "${REGION}" dynamodb update-item \
      --table-name "cla-${STAGE}-github-orgs" \
      --key "{\"organization_name\": {\"S\": \"${1}\"}}" \
      --update-expression 'SET skip_cla = :val' \
      --expression-attribute-values "{\":val\": {\"M\": {\"${2}\":{\"S\":\"${3};${4}\"}}}}"
    ;;
  add-key)
    if ( [ -z "${1}" ] || [ -z "${2}" ] || [ -z "${3}" ] || [ -z "${4}" ] ); then
      echo "Usage: $0 <organization_name> <repo or *> <bot username> <email regexp>"
      exit 1
    fi
    aws --profile "lfproduct-${STAGE}" --region "${REGION}" dynamodb update-item \
      --table-name "cla-${STAGE}-github-orgs" \
      --key "{\"organization_name\": {\"S\": \"${1}\"}}" \
      --update-expression "SET skip_cla.#repo = :val" \
      --expression-attribute-names "{\"#repo\": \"${2}\"}" \
      --expression-attribute-values "{\":val\": {\"S\": \"${3};${4}\"}}"
    ;;
  delete-key)
    if ( [ -z "${1}" ] || [ -z "${2}" ] ); then
      echo "Usage: $0 <organization_name> <repo or *>"
      exit 1
    fi
    aws --profile "lfproduct-${STAGE}" --region "${REGION}" dynamodb update-item \
      --table-name "cla-${STAGE}-github-orgs" \
      --key "{\"organization_name\": {\"S\": \"${1}\"}}" \
      --update-expression "REMOVE skip_cla.#repo" \
      --expression-attribute-names "{\"#repo\": \"${2}\"}"
    ;;
  delete-item)
    if [ -z "${1}" ]; then
      echo "Usage: $0 <organization_name>"
      exit 1
    fi
    aws --profile "lfproduct-${STAGE}" --region "${REGION}" dynamodb update-item \
      --table-name "cla-${STAGE}-github-orgs" \
      --key "{\"organization_name\": {\"S\": \"${1}\"}}" \
      --update-expression "REMOVE skip_cla"
    ;;
  *)
    echo "$0: Unknown MODE: $MODE"
    echo "Valid values are: put-item, add-key, delete-key, delete-item"
    exit 1
    ;;
esac

