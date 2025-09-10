#!/bin/bash
if [ -z "$1" ]
then
  echo "Usage: $0 <ssm-parameter-name>"
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
aws ssm get-parameters --region "${REGION}" --profile "lfproduct-${STAGE}" --names "${1}" --with-decryption
