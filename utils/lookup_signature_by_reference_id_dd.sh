#!/bin/bash
# ./utils/lookup_signature_by_reference_id_dd.sh 3777c5a4-0ca8-11ec-9807-4ebaf2d64a25 | jq -r '.Items[].signature_id'
if [ -z "$1" ]
then
  echo "$0: you need to specify reference_id as a 1st parameter, for example: '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5', '3777c5a4-0ca8-11ec-9807-4ebaf2d64a25'"
  exit 1
fi

if [ -z "$STAGE" ]
then
  STAGE=dev
fi
aws --profile "lfproduct-${STAGE}" dynamodb query --table-name "cla-${STAGE}-signatures" --index-name reference-signature-index --key-condition-expression "signature_reference_id = :reference_id" --expression-attribute-values "{\":reference_id\":{\"S\":\"${1}\"}}" | jq -r '.'
