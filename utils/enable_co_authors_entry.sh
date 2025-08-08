#!/bin/bash
# MODE=mode ./utils/enable_co_authors_entry.sh sun-test-org '*' 'patterns'
# put-item    Overwrites/adds the entire `enable_co_authors` entry.
# add-key     Adds or updates a key/value inside the enable_co_authors map (preserves other keys)
# delete-key  Removes a key from the enable_co_authors map
# delete-item Deletes the entire `enable_co_authors` entry.
#
# MODE=add-key ./utils/enable_co_authors_entry.sh sun-test-org 'repo1' t
# MODE=add-key ./utils/enable_co_authors_entry.sh 'sun-test-org' 're:(?i)^repo[0-9]+$' f
# ./utils/scan.sh github-orgs organization_name sun-test-org
# STAGE=dev DTFROM='1 hour ago' DTTO='1 second ago' ./utils/search_aws_log_group.sh 'cla-backend-dev-githubactivity' 'enable_co_authors'
# MODE=delete-key ./utils/enable_co_authors_entry.sh 'sun-test-org' 're:(?i)^repo[0-9]+$'
# STAGE=dev MODE=add-key DEBUG=1 ./utils/enable_co_authors_entry.sh 'sun-test-org' 'repo1' f
# STAGE=dev MODE=add-key ./utils/enable_co_authors_entry.sh 'open-telemetry' '*' t
# STAGE=dev MODE=add-key ./utils/enable_co_authors_entry.sh 'openfga' 'vscode-ext' t
# STAGE=prod MODE=add-key DEBUG=1 ./utils/enable_co_authors_entry.sh 'open-telemetry' 'opentelemetry-rust' t
# STAGE=prod MODE=add-key ./utils/enable_co_authors_entry.sh 'openfga' 'vscode-ext' t

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
      echo "Usage: $0 <organization_name> <repo or re:repo-regexp or *> <t or f>>"
      exit 1
    fi
    repo=$(echo "${2}" | sed 's/\\/\\\\/g')
    if [[ "${3}" == "t" ]]
    then
      b=true
    elif [[ "${3}" == "f" ]]
    then
      b=false
    else
      echo "$0: Third parameter '${3}' must be 't' or 'f'"
      exit 1
    fi
    CMD="aws --profile \"lfproduct-${STAGE}\" --region \"${REGION}\" dynamodb update-item \
      --table-name \"cla-${STAGE}-github-orgs\" \
      --key '{\"organization_name\": {\"S\": \"${1}\"}}' \
      --update-expression 'SET enable_co_authors = :val' \
      --expression-attribute-values '{\":val\": {\"M\": {\"${repo}\": {\"BOOL\": ${b}}}}}'"
    ;;
  add-key)
    if ( [ -z "${1}" ] || [ -z "${2}" ] || [ -z "${3}" ] ); then
      echo "Usage: $0 <organization_name> <repo or re:repo-regexp or *> <t or f>"
      exit 1
    fi
    repo=$(echo "${2}" | sed 's/\\/\\\\/g')
    if [[ "${3}" == "t" ]]
    then
      b=true
    elif [[ "${3}" == "f" ]]
    then
      b=false
    else
      echo "$0: Third parameter '${3}' must be 't' or 'f'"
      exit 1
    fi
    CMD="aws --profile \"lfproduct-${STAGE}\" --region \"${REGION}\" dynamodb update-item \
      --table-name \"cla-${STAGE}-github-orgs\" \
      --key '{\"organization_name\": {\"S\": \"${1}\"}}' \
      --update-expression 'SET enable_co_authors.#repo = :val' \
      --expression-attribute-names '{\"#repo\": \"${repo}\"}' \
      --expression-attribute-values '{\":val\": {\"BOOL\": ${b}}}'"
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
      --update-expression 'REMOVE enable_co_authors.#repo' \
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
      --update-expression 'REMOVE enable_co_authors'"
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

