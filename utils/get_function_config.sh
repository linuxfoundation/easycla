#!/bin/bash

if [ -z "$1" ]
then
  echo "Usage: $0 <function-name>"
  exit 1
fi

if [ -z "$REGION" ]
then
  REGION="us-east-2"
fi

if [ -z "$STAGE" ]
then
  STAGE="dev"
fi

aws lambda get-function-configuration --function-name "${1}" --region "${REGION}" --profile "lfproduct-${STAGE}"
