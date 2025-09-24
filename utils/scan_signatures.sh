#!/bin/bash
# add: | jq -r '.[].signature_acl'
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
if [ -z "$1" ]
then
  aws --profile "lfproduct-${STAGE}" dynamodb scan --table-name "cla-${STAGE}-signatures" --max-items 100 | jq -r '.Items'
else
  if [ -n "${DEBUG}" ]
  then
    echo "aws --profile lfproduct-${STAGE} dynamodb scan --table-name cla-${STAGE}-signatures --filter-expression \"contains(${1}, :v)\" --expression-attribute-values '{\":v\":{\"S\":\"${2}\"}}' --max-items 100"
  fi
  aws --profile "lfproduct-${STAGE}" dynamodb scan --table-name "cla-${STAGE}-signatures" --filter-expression "contains(${1},:v)" --expression-attribute-values "{\":v\":{\"S\":\"${2}\"}}" --max-items 100 | jq -r '.Items'
fi
