#!/bin/bash
# STAGE=dev|prod
# DEBUG=1
# example update: STAGE=prod DEBUG=1 ./utils/update_company_is_sanctioned.sh "0ca30016-6457-466c-bc41-a09560c1f9bf" false
if [ -z "$STAGE" ]
then
  export STAGE=dev
fi
if [ -z "$1" ]
then
  echo "$0: you need to specify company_id, for example: '0ca30016-6457-466c-bc41-a09560c1f9bf'"
  exit 1
fi
if [ -z "$2" ]
then
  echo "$0: you need to value: true|false"
  exit 2
fi
if [ ! -z "$DEBUG" ]
then
  echo aws --profile "lfproduct-$STAGE" dynamodb update-item --table-name "cla-${STAGE}-companies" --key "{\"company_id\":{\"S\":\"${1}\"}}" --update-expression '"SET is_sanctioned = :val"' --expression-attribute-values "{\":val\":{\"BOOL\":${2}}}"
fi
aws --profile "lfproduct-$STAGE" dynamodb update-item --table-name "cla-${STAGE}-companies" --key "{\"company_id\":{\"S\":\"${1}\"}}" --update-expression "SET is_sanctioned = :val" --expression-attribute-values "{\":val\":{\"BOOL\":${2}}}"
