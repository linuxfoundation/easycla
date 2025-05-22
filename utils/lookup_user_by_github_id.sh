#!/bin/bash
if [ -z "$1" ]
then
  echo "$0: you need to specify GitHub ID as a 1st parameter, for example: 2469783"
  exit 1
fi
if [ -z "${STAGE}" ]
then
  export STAGE=dev
fi

if [ ! -z "${DEBUG}" ]
then
  echo "aws --profile \"lfproduct-${STAGE}\" dynamodb query --table-name \"cla-${STAGE}-users\" --index-name github-id-index --key-condition-expression \"user_github_id = :github_id\" --expression-attribute-values '{\":github_id\":{\"N\":\"${1}\"}}' | jq -r '.'"
fi
aws --profile "lfproduct-${STAGE}" dynamodb query --table-name "cla-${STAGE}-users" --index-name github-id-index --key-condition-expression "user_github_id = :github_id" --expression-attribute-values "{\":github_id\":{\"N\":\"${1}\"}}" | jq -r '.'
