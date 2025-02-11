#!/bin/bash
# add: | jq -r '.[].signature_acl'
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
if [ -z "$1" ]
then
  aws --profile "lfproduct-${STAGE}" dynamodb scan --table-name "cla-${STAGE}-users" --max-items 100 | jq -r '.Items'
else
  aws --profile "lfproduct-${STAGE}" dynamodb scan --table-name "cla-${STAGE}-users" --filter-expression "contains(${1},:v)" --expression-attribute-values "{\":v\":{\"S\":\"${2}\"}}" --max-items 100 | jq -r '.Items'
fi
