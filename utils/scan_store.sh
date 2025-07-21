#!/bin/bash
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
if [ ! -z "${DEBUG}" ]
then
  echo "aws --profile \"lfproduct-${STAGE}\" dynamodb scan --table-name \"cla-${STAGE}-store\" --max-items 100 | jq -r '.Items'"
fi
aws --profile "lfproduct-${STAGE}" dynamodb scan --table-name "cla-${STAGE}-store" --max-items 100 | jq -r '.Items'
