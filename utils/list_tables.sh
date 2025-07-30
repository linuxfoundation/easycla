#!/bin/bash
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
if [ -z "$REGION" ]
then
  REGION=us-east-1
fi
aws --profile "lfproduct-${STAGE}" --region "${REGION}" dynamodb list-tables | grep "cla-${STAGE}-"
