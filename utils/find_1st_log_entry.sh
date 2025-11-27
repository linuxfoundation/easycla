#!/bin/bash
# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT
if [ -z "${STAGE}" ]
then
  export STAGE=dev
fi
# Array of log group configurations: "region log_group_name"
log_groups=(
  "us-east-1 /aws/lambda/cla-backend-${STAGE}-api-v3-lambda"
  "us-east-1 /aws/lambda/cla-backend-${STAGE}-apiv1"
  "us-east-1 /aws/lambda/cla-backend-${STAGE}-apiv2"
  "us-east-1 /aws/lambda/cla-backend-${STAGE}-githubactivity"
  "us-east-1 /aws/lambda/cla-backend-${STAGE}-githubinstall"
  "us-east-2 /aws/lambda/cla-backend-go-api-v4-lambda"
)

for entry in "${log_groups[@]}"; do
  region=$(echo "$entry" | awk '{print $1}')
  log_group=$(echo "$entry" | cut -d' ' -f2-)
  aws --region "$region" --profile "lfproduct-${STAGE}" logs filter-log-events \
    --log-group-name "$log_group" \
    --start-time 0 --limit 1 --filter-pattern "\"LG:api-request-path\"" \
    | jq -r '.events.[0].timestamp' \
    | awk '{print strftime("%Y-%m-%d %H:%M:%S", $1/1000)}'
done
