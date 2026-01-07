#!/bin/bash
if [ -z "$AWS_REGION" ]; then
  echo "AWS_REGION is not set. Please set it before running the script."
  exit 1
fi
if [ -z "$STAGE" ]; then
  echo "STAGE is not set. Please set it before running the script."
  exit 1
fi
export TABLE_NAME=cla-${STAGE}-api-log

aws --profile "lfproduct-${STAGE}" \
    dynamodb create-table \
  --table-name ${TABLE_NAME} \
  --attribute-definitions \
    AttributeName=url,AttributeType=S \
    AttributeName=dt,AttributeType=N \
  --key-schema \
    AttributeName=url,KeyType=HASH \
    AttributeName=dt,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST \
  --region ${AWS_REGION}

aws --profile "lfproduct-${STAGE}" \
    dynamodb wait table-exists \
  --table-name ${TABLE_NAME} \
  --region ${AWS_REGION}

aws --profile "lfproduct-${STAGE}" \
    dynamodb update-table \
  --table-name  ${TABLE_NAME} \
  --attribute-definitions \
    AttributeName=dt,AttributeType=N \
    AttributeName=url,AttributeType=S \
  --global-secondary-index-updates '[
    {
      "Create": {
        "IndexName": "dt-index",
        "KeySchema": [
          {"AttributeName": "dt", "KeyType": "HASH"},
          {"AttributeName": "url", "KeyType": "RANGE"}
        ],
        "Projection": {"ProjectionType": "ALL"}
      }
    }
  ]' \
  --region ${AWS_REGION}

