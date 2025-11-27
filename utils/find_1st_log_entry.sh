#!/bin/bash
if [ -z "${STAGE}" ]
then
  export STAGE=dev
fi
aws --region us-east-1 --profile "lfproduct-${STAGE}" logs filter-log-events --log-group-name "/aws/lambda/cla-backend-${STAGE}-api-v3-lambda" --start-time 0 --limit 1 --filter-pattern "\"LG:api-request-path\"" | jq -r '.events.[0].timestamp' | awk '{print strftime("%Y-%m-%d %H:%M:%S", $1/1000)}'
aws --region us-east-1 --profile "lfproduct-${STAGE}" logs filter-log-events --log-group-name "/aws/lambda/cla-backend-${STAGE}-apiv1" --start-time 0 --limit 1 --filter-pattern "\"LG:api-request-path\"" | jq -r '.events.[0].timestamp' | awk '{print strftime("%Y-%m-%d %H:%M:%S", $1/1000)}'
aws --region us-east-1 --profile "lfproduct-${STAGE}" logs filter-log-events --log-group-name "/aws/lambda/cla-backend-${STAGE}-apiv2" --start-time 0 --limit 1 --filter-pattern "\"LG:api-request-path\"" | jq -r '.events.[0].timestamp' | awk '{print strftime("%Y-%m-%d %H:%M:%S", $1/1000)}'
aws --region us-east-1 --profile "lfproduct-${STAGE}" logs filter-log-events --log-group-name "/aws/lambda/cla-backend-${STAGE}-githubactivity" --start-time 0 --limit 1 --filter-pattern "\"LG:api-request-path\"" | jq -r '.events.[0].timestamp' | awk '{print strftime("%Y-%m-%d %H:%M:%S", $1/1000)}'
aws --region us-east-1 --profile "lfproduct-${STAGE}" logs filter-log-events --log-group-name "/aws/lambda/cla-backend-${STAGE}-githubinstall" --start-time 0 --limit 1 --filter-pattern "\"LG:api-request-path\"" | jq -r '.events.[0].timestamp' | awk '{print strftime("%Y-%m-%d %H:%M:%S", $1/1000)}'
aws --region us-east-2 --profile "lfproduct-${STAGE}" logs filter-log-events --log-group-name "/aws/lambda/cla-backend-go-api-v4-lambda" --start-time 0 --limit 1 --filter-pattern "\"LG:api-request-path\"" | jq -r '.events.[0].timestamp' | awk '{print strftime("%Y-%m-%d %H:%M:%S", $1/1000)}'
