#!/bin/bash

if [ -z "$AWS_REGION" ]; then
  echo "AWS_REGION is not set. Please set it before running the script."
  exit 1
fi
if [ -z "$STAGE" ]; then
  echo "STAGE is not set. Please set it before running the script."
  exit 1
fi

aws --profile lfproduct-${STAGE} --region ${AWS_REGION} \
    iam get-role-policy \
  --role-name cla-backend-${STAGE}-${AWS_REGION}-lambdaRole \
  --policy-name cla-backend-${STAGE}-lambda \
  --query PolicyDocument \
  --output json > /tmp/cla-backend-${STAGE}-${AWS_REGION}-lambda.json

vim /tmp/cla-backend-${STAGE}-${AWS_REGION}-lambda.json

aws --profile lfproduct-${STAGE} --region ${AWS_REGION} \
    iam put-role-policy \
  --role-name cla-backend-${STAGE}-${AWS_REGION}-lambdaRole \
  --policy-name cla-backend-${STAGE}-lambda \
  --policy-document file:///tmp/cla-backend-${STAGE}-${AWS_REGION}-lambda.json
