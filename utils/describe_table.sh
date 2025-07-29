#!/bin/bash
if [ -z "$1" ]
then
  echo "$0: you need to specify table as a 1st parameter, for example: 'users'"
  exit 1
fi
if [ -z "${STAGE}" ]
then
  export STAGE=dev
fi
if [ -z "$REGION" ]
then
  REGION=us-east-1
fi
if [ ! -z "${DEBUG}" ]
then
  echo "aws --profile \"lfproduct-${STAGE}\" --region \"${REGION}\" dynamodb describe-table --table-name \"cla-${STAGE}-${1}\""
  aws --profile "lfproduct-${STAGE}" --region "${REGION}" dynamodb describe-table --table-name "cla-${STAGE}-${1}"
else
  aws --profile "lfproduct-${STAGE}" --region "${REGION}" dynamodb describe-table --table-name "cla-${STAGE}-${1}" | jq -r '.Table.AttributeDefinitions'
fi
