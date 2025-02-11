#!/bin/bash
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
if [ -z "$1" ]
then
  aws --profile lfproduct-${STAGE} dynamodb scan --table-name cla-${STAGE}-projects --max-items 100
else
  aws --profile lfproduct-${STAGE} dynamodb scan --table-name cla-${STAGE}-projects --filter-expression "contains(${1},:v)" --expression-attribute-values "{\":v\":{\"S\":\"${2}\"}}" --max-items 100
fi
