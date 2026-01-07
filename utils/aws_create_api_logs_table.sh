#!/usr/bin/env bash
set -euo pipefail

: "${AWS_REGION:?AWS_REGION is not set. Example: us-east-1}"
: "${STAGE:?STAGE is not set. Example: dev}"

PROFILE="lfproduct-${STAGE}"
TABLE_NAME="cla-${STAGE}-api-log"

echo "Creating table: ${TABLE_NAME} in ${AWS_REGION} using profile ${PROFILE}"

# 1) Create table (ONLY define attrs used by the table key schema)
aws --profile "${PROFILE}" dynamodb create-table \
  --table-name "${TABLE_NAME}" \
  --attribute-definitions \
    AttributeName=url,AttributeType=S \
    AttributeName=dt,AttributeType=N \
  --key-schema \
    AttributeName=url,KeyType=HASH \
    AttributeName=dt,KeyType=RANGE \
  --billing-mode PAY_PER_REQUEST \
  --region "${AWS_REGION}"

aws --profile "${PROFILE}" dynamodb wait table-exists \
  --table-name "${TABLE_NAME}" \
  --region "${AWS_REGION}"

echo "Creating GSI bucket-dt-index (supports time-range queries across all URLs)"

# 2) Add GSI: bucket (HASH) + dt (RANGE)
aws --profile "${PROFILE}" dynamodb update-table \
  --table-name "${TABLE_NAME}" \
  --attribute-definitions \
    AttributeName=bucket,AttributeType=S \
    AttributeName=dt,AttributeType=N \
  --global-secondary-index-updates '[
    {
      "Create": {
        "IndexName": "bucket-dt-index",
        "KeySchema": [
          { "AttributeName": "bucket", "KeyType": "HASH" },
          { "AttributeName": "dt", "KeyType": "RANGE" }
        ],
        "Projection": { "ProjectionType": "ALL" }
      }
    }
  ]' \
  --region "${AWS_REGION}"

echo "Waiting for GSI to become ACTIVE..."
# Wait until the index becomes ACTIVE (polling)
while true; do
  STATUS=$(aws --profile "${PROFILE}" dynamodb describe-table \
    --table-name "${TABLE_NAME}" \
    --region "${AWS_REGION}" \
    --query "Table.GlobalSecondaryIndexes[?IndexName=='bucket-dt-index'].IndexStatus | [0]" \
    --output text)
  echo "bucket-dt-index status: ${STATUS}"
  if [[ "${STATUS}" == "ACTIVE" ]]; then
    break
  fi
  sleep 5
done

aws --profile ${PROFILE} dynamodb describe-table --table-name cla-${STAGE}-api-log --region ${AWS_REGION}

echo "Done."
