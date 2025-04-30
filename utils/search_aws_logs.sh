#!/bin/bash
# STAGE=dev DEBUG=1 DTFROM='3 days ago' DTTO='2 days ago' ./utils/search_aws_logs.sh 'cla-backend-dev-githubactivity' 'error'

if [ -z "$STAGE" ]
then
  export STAGE=dev
fi

if [ -z "${1}" ]
then
  echo "$0: you must specify log group name, for example: 'cla-backend-dev-githubactivity', 'cla-backend-dev-apiv2', 'cla-backend-dev-api-v3-lambda', 'cla-backend-go-api-v4-lambda'"
  exit 1
fi

log_group=$(echo "$1" | sed -E "s/\b(dev|prod)\b/${STAGE}/g")

if [ -z "${2}" ]
then
  echo "$0: you must specify the search term, for example 'error'"
  exit 2
fi

if [ -z "${DTFROM}" ]
then
  export DTFROM="$(date -d '3 days ago' +%s)000"
else
  export DTFROM="$(date -d "${DTFROM}" +%s)000"
fi

if [ -z "${DTTO}" ]
then
  export DTTO="$(date +%s)000"
else
  export DTTO="$(date -d "${DTTO}" +%s)000"
fi

if [ ! -z "${DEBUG}" ]
then
  echo "aws --profile \"lfproduct-${STAGE}\" logs filter-log-events --log-group-name \"/aws/lambda/${log_group}\" --start-time \"${DTFROM}\" --end-time \"${DTTO}\" --filter-pattern \"${2}\""
  aws --profile "lfproduct-${STAGE}" logs filter-log-events --log-group-name "/aws/lambda/${log_group}" --start-time "${DTFROM}" --end-time "${DTTO}" --filter-pattern "\"${2}\""
else
  aws --profile "lfproduct-${STAGE}" logs filter-log-events --log-group-name "/aws/lambda/${log_group}" --start-time "${DTFROM}" --end-time "${DTTO}" --filter-pattern "\"${2}\"" | jq -r '.events'
fi

