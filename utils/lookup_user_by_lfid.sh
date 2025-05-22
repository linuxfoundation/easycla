#!/bin/bash
if [ -z "$1" ]
then
  echo "$0: you need to specify lfid as a 1st parameter, for example: 'lgryglicki'"
  exit 1
fi
if [ -z "${STAGE}" ]
then
  export STAGE=dev
fi

if [ ! -z "${DEBUG}" ]
then
  echo "aws --profile \"lfproduct-${STAGE}\" dynamodb query --table-name \"cla-${STAGE}-users\" --index-name lf-username-index --key-condition-expression \"lf_username = :name\" --expression-attribute-values '{\":name\":{\"S\":\"${1}\"}}' | jq -r '.'"
fi
aws --profile "lfproduct-${STAGE}" dynamodb query --table-name "cla-${STAGE}-users" --index-name lf-username-index --key-condition-expression "lf_username = :name" --expression-attribute-values "{\":name\":{\"S\":\"${1}\"}}" | jq -r '.'
