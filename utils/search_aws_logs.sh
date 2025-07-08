#!/bin/bash

# Copyright The Linux Foundation and each contributor to LFX.
# SPDX-License-Identifier: MIT

# REGION=us-east-1|us-east-2 STAGE=dev DEBUG=1 DTFROM='3 days ago' DTTO='2 days ago' OUT=logs.json ./utils/search_aws_logs.sh 'error'
# DEBUG=1 STAGE=dev REGION=us-east-1 DTFROM='10 days ago' DTTO='1 second ago' OUT=api-logs-dev.json ./utils/search_aws_logs.sh 'LG:api-request-path' && jq -r '.[].message' api-logs-dev.json | grep -o 'LG:api-request-path:[^[:space:]]*' | sed 's/^LG:api-request-path://' | sed -E 's/[0-9a-fA-F-]{36}/<uuid>/g' | sed -E 's/\b[0-9]{2,}\b/<id>/g' | sort | uniq -c | sort -nr
# DEBUG=1 STAGE=prod REGION=us-east-1 NO_ECHO=1 DTFROM='10 days ago' DTTO='1 second ago' OUT=api-logs-prod.json ./utils/search_aws_logs.sh 'LG:api-request-path' && jq -r '.[].message' api-logs-prod.json | grep -o 'LG:api-request-path:[^[:space:]]*' | sed 's/^LG:api-request-path://' | sed -E 's/[0-9a-fA-F-]{36}/<uuid>/g' | sed -E ':a;s#/([0-9]{1,})(/|$)#/<id>\2#g;ta' | sort | uniq -c | sort -nr
# To find distinct log groups: | jq -r 'map(.logGroupName) | unique | .[]'
# in us-east-1 (mostly V1, V2 and V3):
# To see specific log group: | jq 'map(select(.logGroupName == "/aws/lambda/cla-backend-dev-apiv1"))'
# To filter out one log group: | jq 'map(select(.logGroupName != "/aws/lambda/cla-backend-dev-apiv2"))'
# To filter out one log group: | jq 'map(select(.logGroupName != "cla-backend-dev-api-v3-lambda"))'
# in us-east-2 (V4):
# To filter out one log group: | jq 'map(select(.logGroupName != "cla-backend-go-api-v4-lambda"))'
# To exclude some log groups: EXCL_LOGS="apiv1,apiv3"
# To include only specific log groups INCL_LOGS="apiv1,apiv3"
# All log groups in us-east-1: backend-dev-api-v3-lambda,backend-dev-apiv1,backend-dev-apiv2,backend-dev-authorizer,backend-dev-dynamo-events-events-lambda,backend-dev-dynamo-github-orgs-events-lambda,backend-dev-dynamo-projects-cla-groups-events-lambda,backend-dev-dynamo-projects-lambda,backend-dev-dynamo-repositories-events-lambda,backend-dev-dynamo-signatures-events-lambda,backend-dev-githubactivity,backend-dev-githubinstall,backend-dev-gitlab-repository-check-lambda,backend-dev-report-metrics-lambda,backend-dev-salesforceprojectbyID,backend-dev-salesforceprojects,backend-dev-save-metrics-lambda,backend-dev-user-event-handler-lambda,backend-dev-zip-builder-lambda,backend-dev-zip-builder-scheduler-lambda,backend-go-api,dev-stream-test-handler,dynamo-events-lambda,landing-page-dev-clientEdge,metrics-lamdba-test,prasanna-zip-builder,prasanna-zipbuilder-schedular,test-zipbuilder-lambda
# All log groups in us-east-2: backend-go-api-v4-lambda

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

IFS=',' read -ra INCL_LOGS_ARRAY <<< "${INCL_LOGS}"
IFS=',' read -ra EXCL_LOGS_ARRAY <<< "${EXCL_LOGS}"

results=()
for log_group in "${log_groups_array[@]}"
do
  short_log_group="${log_group#/aws/lambda/cla-}"
  # Skip if not in INCLUDE list (when it's set)
  if [ ! -z "${INCL_LOGS}" ]
  then
    skip_included=true
    for incl in "${INCL_LOGS_ARRAY[@]}"
    do
      if [[ "$short_log_group" == *"$incl"* ]]
      then
        skip_included=false
        break
      fi
    done
    if [ "$skip_included" = true ]
    then
      [ ! -z "$DEBUG" ] && echo "Skipping (not in INCLUDE): $log_group" >&2
      continue
    fi
  fi

  # Skip if in EXCLUDE list
  for excl in "${EXCL_LOGS_ARRAY[@]}"
  do
    if [[ "$short_log_group" == *"$excl"* ]]
    then
      [ ! -z "$DEBUG" ] && echo "Skipping (in EXCLUDE): $log_group" >&2
      continue 2
    fi
  done

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
    if [ ! -z "$jsons" ]
    then
        jsons+=$'\n'
    fi
    jsons+=$(echo "$log" | jq -r '.')
done

if [ ! -z "${OUT}" ]
then
  echo "$jsons" | jq -s 'sort_by(.dt) | reverse' > "${OUT}"
fi
if [ -z "${NO_ECHO}" ]
then
  echo "$jsons" | jq -s 'sort_by(.dt) | reverse'
fi
