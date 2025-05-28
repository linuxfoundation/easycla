#!/bin/bash

# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT

# REGION=us-east-1|us-east-2 STAGE=dev DEBUG=1 DTFROM='3 days ago' DTTO='2 days ago' ./utils/search_aws_logs.sh 'error'
# To find distinct log groups: | jq -r 'map(.logGroupName) | unique | .[]'
# in us-east-1 (mostly V1, V2 and V3):
# To see specific log group: | jq 'map(select(.logGroupName == "/aws/lambda/cla-backend-dev-apiv1"))'
# To filter out one log group: | jq 'map(select(.logGroupName != "/aws/lambda/cla-backend-dev-apiv2"))'
# To filter out one log group: | jq 'map(select(.logGroupName != "cla-backend-dev-api-v3-lambda"))'
# in us-east-2 (V4):
# To filter out one log group: | jq 'map(select(.logGroupName != "cla-backend-go-api-v4-lambda"))'

if [ -z "${STAGE}" ]
then
  export STAGE=prod
fi

if [ -z "${REGION}" ]
then
  export REGION="us-east-1"
fi

search="${1}"
if [ -z "${1}" ]
then
  echo "$0: you should specify the search term, defaulting to 'error'"
  search="error"
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

mapfile -t log_groups_array < <(aws --region "${REGION}" --profile "lfproduct-${STAGE}" logs describe-log-groups --log-group-name-prefix "/aws/lambda/cla-" --query "logGroups[].logGroupName" | jq -r '.[]')
results=()
for log_group in "${log_groups_array[@]}"
do
  if [ ! -z "${DEBUG}" ]
  then
    echo "lookup log group '${log_group}': aws --region "${REGION}" --profile \"lfproduct-${STAGE}\" logs filter-log-events --log-group-name \"$log_group\" --start-time \"${DTFROM}\" --end-time \"${DTTO}\" --filter-pattern \"${search}\"" >&2
  fi
  logs=$(aws --region "${REGION}" --profile "lfproduct-${STAGE}" logs filter-log-events \
  --log-group-name "$log_group" \
  --start-time "${DTFROM}" \
  --end-time "${DTTO}" \
  --filter-pattern "\"${search}\"" | jq --arg logGroupName "$log_group" '
  .events[] |
  .logGroupName = $logGroupName |
  .dt = ( (.timestamp / 1000) | strftime("%Y-%m-%d %H:%M:%S") ) + "." + ( (.timestamp % 1000 | tostring) | if length == 1 then "00" + . elif length == 2 then "0" + . else . end )
  ')
  if [ ! -z "$logs" ]
  then
    results+=("${logs}")
  fi
done

jsons=""
for log in "${results[@]}"; do
    if [ -n "$jsons" ]
    then
        jsons+=$'\n'
    fi
    jsons+=$(echo "$log" | jq -r '.')
done

echo "$jsons" | jq -s 'sort_by(.dt) | reverse'
