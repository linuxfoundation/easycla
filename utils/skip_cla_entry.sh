#!/bin/bash
# MODE=mode ./utils/skip_cla_entry.sh sun-test-org '*' 'copilot-swe-agent[bot]' '*'
# put-item    Overwrites/adds the entire `skip_cla` entry.
# add-key     Adds or updates a key/value inside the skip_cla map (preserves other keys)
# delete-key  Removes a key from the skip_cla map
# delete-item Deletes the entire `skip_cla` entry.
#
# MODE=add-key ./utils/skip_cla_entry.sh sun-test-org 'repo1' 're:vee?rendra;*;*'
# MODE=add-key ./utils/skip_cla_entry.sh 'sun-test-org' 'repo1' 'lukaszgryglicki;re:gryglicki'
# MODE=add-key ./utils/skip_cla_entry.sh 'sun-test-org' 're:(?i)^repo[0-9]+$' '[re:(?i)^l(ukasz)?gryglicki$;re:(?i)^l(ukasz)?gryglicki@;*||copilot-swe-agent[bot]]'
# ./utils/scan.sh github-orgs organization_name sun-test-org
# STAGE=dev DTFROM='1 hour ago' DTTO='1 second ago' ./utils/search_aws_log_group.sh 'cla-backend-dev-githubactivity' 'skip_cla'
# MODE=delete-key ./utils/skip_cla_entry.sh 'sun-test-org' 're:(?i)^repo[0-9]+$'
# STAGE=dev MODE=add-key DEBUG=1 ./utils/skip_cla_entry.sh 'sun-test-org' 'repo1' 'thakurveerendras;;*'
# STAGE=prod MODE=add-key DEBUG=1 ./utils/skip_cla_entry.sh 'open-telemetry' 'opentelemetry-rust' '*;re:^\d+\+Copilot@users\.noreply\.github\.com$;copilot-swe-agent[bot]'

if [ -z "$MODE" ]
then
  echo "$0: MODE must be set, valid values are: put-item, add-key, delete-key, delete-item"
  exit 1
fi

if [ -z "$STAGE" ]; then
  STAGE='dev'
fi
if [ -z "$REGION" ]; then
  REGION='us-east-1'
fi

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
  *)
    echo "$0: Unknown MODE: $MODE"
    echo "Valid values are: put-item, add-key, delete-key, delete-item"
    exit 1
    ;;
esac

if [ ! -z "$DEBUG" ]
then
  echo "$CMD"
fi

eval $CMD

