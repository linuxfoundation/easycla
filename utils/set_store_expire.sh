#!/bin/bash
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
if [ -z "${1}" ]
then
  echo "$0: you must specify ID value as a 1st argument, like: 'b817eb57-045a-4fe0-8473-fbb416a01d70'"
  exit 1
fi
if [ ! -z "${DEBUG}" ]
then
  echo "aws --profile \"lfproduct-${STAGE}\" dynamodb update-item --table-name \"cla-${STAGE}-store\" --key '{\"key\": {\"S\": \"${1}\"}}' --update-expression \"SET expire = :newval\" --expression-attribute-values '{\":newval\": {\"N\": \"'\"$(date -d '+1 day' +%s)\"'\"}}'"
fi
aws --profile "lfproduct-${STAGE}" dynamodb update-item --table-name "cla-${STAGE}-store" --key "{\"key\": {\"S\": \"${1}\"}}" --update-expression "SET expire = :newval" --expression-attribute-values '{":newval": {"N": "'"$(date -d '+1 day' +%s)"'"}}'
