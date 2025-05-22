#!/bin/bash
# STAGE=prod ./utils/lookup_store_ddb.sh 'active_signature:564e571e-12d7-4857-abd4-898939accdd7'
if [ -z "$1" ]
then
  echo "$0: you need to specify key as a 1st argument"
  echo "example: 'active_signature:564e571e-12d7-4857-abd4-898939accdd7'"
  exit 1
fi
if [ -z "${STAGE}" ]
then
  export STAGE=dev
fi

if [ ! -z "${DEBUG}" ]
then
  echo "aws --profile \"lfproduct-${STAGE}\" dynamodb get-item --table-name \"cla-${STAGE}-store\" --key '{\"key\": {\"S\": \"${1}\"}}' | jq -r '.'"
fi
aws --profile "lfproduct-${STAGE}" dynamodb get-item --table-name "cla-${STAGE}-store" --key "{\"key\": {\"S\": \"${1}\"}}" | jq -r '.'
