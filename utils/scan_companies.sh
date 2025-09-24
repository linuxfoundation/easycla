#!/bin/bash
# DEBUG=1
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
if [ -z "$1" ]
then
  aws --profile "lfproduct-${STAGE}" dynamodb scan --table-name "cla-${STAGE}-companies" --max-items 100
else
  if [ -n "${DEBUG}" ]
  then
    echo "aws --profile lfproduct-${STAGE} dynamodb scan --table-name cla-${STAGE}-companies --filter-expression \"contains(${1}, :v)\" --expression-attribute-values '{\":v\":{\"S\":\"${2}\"}}' --max-items 100"
  fi
  aws --profile "lfproduct-${STAGE}" dynamodb scan --table-name "cla-${STAGE}-companies" --filter-expression "contains(${1}, :v)" --expression-attribute-values "{\":v\":{\"S\":\"${2}\"}}" --max-items 100
fi

