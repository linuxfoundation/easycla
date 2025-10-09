#!/bin/bash
# STAGE=dev DEBUG=1 DTFROM='3 days ago' DTTO='2 days ago' ./utils/search_aws_log_group.sh 'cla-backend-dev-githubactivity' 'error'
# STAGE=dev DEBUG=1 DTFROM='3 days ago' DTTO='2 days ago' ./utils/search_aws_log_group.sh 'cla-backend-dev-githubactivity' 'Runtime exited with'
# REGION=us-east-2 STAGE=prod DEBUG=1 DTFROM='15 minutes ago' DTTO='1 second ago' ./utils/search_aws_log_group.sh 'cla-backend-go-api-v4-lambda' 'LG:api-request-path'
# REGION=us-east-1 STAGE=prod DEBUG=1 DTFROM='15 minutes ago' DTTO='1 second ago' ./utils/search_aws_log_group.sh 'cla-backend-prod-api-v3-lambda' 'LG:api-request-path'
# REGION=us-east-1 STAGE=prod DEBUG=1 DTFROM='15 minutes ago' DTTO='1 second ago' ./utils/search_aws_log_group.sh 'cla-backend-prod-apiv2' 'LG:api-request-path'
# REGION=us-east-1 STAGE=prod DEBUG=1 DTFROM='15 minutes ago' DTTO='1 second ago' ./utils/search_aws_log_group.sh 'cla-backend-prod-githubactivity' 'LG:api-request-path'

if [ -z "$STAGE" ]
then
  export STAGE=dev
fi

if [ -z "${REGION}" ]
then
  export REGION="us-east-1"
fi

if [ -z "${1}" ]
then
  echo "$0: you must specify log group name, for example: 'cla-backend-dev-githubactivity', 'cla-backend-prod-apiv2', 'cla-backend-dev-api-v3-lambda', 'cla-backend-go-api-v4-lambda'"
  echo "or short group name: 'githubactivity', 'apiv2', 'api-v3-lambda'"
  exit 1
fi

log_group=$(echo "$1" | sed -E "s/\b(dev|prod)\b/${STAGE}/g")

if [[ ! "$log_group" =~ ^cla-backend- ]]
then
  log_group="cla-backend-${STAGE}-$log_group"
fi

if [ -z "${2}" ]
then
  echo "$0: you must specify the search term, for example 'Runtime exited with'"
  exit 2
fi

to_epoch_ms () {
  local v="$1"
  if [[ "$v" =~ ^[0-9]{13}$ ]]
  then
    echo "$v"; return
  fi
  if [[ "$v" =~ ^[0-9]{10}$ ]]
  then
    echo "${v}000"; return
  fi
  v="${v/T/ }"
  v="${v/Z/ UTC}"
  echo "$(date -d "$v" +%s)000"
}

if [ -z "${DTFROM}" ]
then
  export DTFROM="$(to_epoch_ms '3 days ago')"
else
  export DTFROM="$(to_epoch_ms "${DTFROM}")"
fi

# DTTO
if [ -z "${DTTO}" ]
then
  export DTTO="$(to_epoch_ms 'now')"
else
  export DTTO="$(to_epoch_ms "${DTTO}")"
fi

DTF=$(date -u -d @$(echo "${DTFROM}/1000" | bc) "+%F %T.%6N")
DTT=$(date -u -d @$(echo "${DTTO}/1000" | bc) "+%F %T.%6N")
echo "Date range: ${DTF} .. ${DTT} (from ${DTFROM} to ${DTTO})"

if [ ! -z "${DEBUG}" ]
then
  echo "aws --region \"${REGION}\" --profile \"lfproduct-${STAGE}\" logs filter-log-events --log-group-name \"/aws/lambda/${log_group}\" --start-time \"${DTFROM}\" --end-time \"${DTTO}\" --filter-pattern \"${2}\""
fi
aws --region "${REGION}" --profile "lfproduct-${STAGE}" logs filter-log-events --log-group-name "/aws/lambda/${log_group}" --start-time "${DTFROM}" --end-time "${DTTO}" --filter-pattern "\"${2}\"" | jq -r '.events | sort_by(.timestamp)'

