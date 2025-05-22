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

if [ ! -z "${DEBUG}" ]
then
  echo "aws --profile \"lfproduct-${STAGE}\" dynamodb describe-table --table-name \"cla-${STAGE}-${1}\""
fi
aws --profile "lfproduct-${STAGE}" dynamodb describe-table --table-name "cla-${STAGE}-${1}"
