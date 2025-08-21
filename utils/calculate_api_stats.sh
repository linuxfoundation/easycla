#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT


if [ -z "$STAGE" ]
then
  export STAGE=prod
fi
if [ -z "$1" ]
then
  echo "$0: please provide time range from value as a 1st argument, for example '2 hours ago'"
  exit 1
fi
export DTFROM="${1}"
REGION=us-east-1 NO_ECHO=1 DTTO='1 second ago' OUT="api-logs-${STAGE}-1.json" ./utils/search_aws_logs.sh 'LG:api-request-path'
REGION=us-east-2 NO_ECHO=1 DTTO='1 second ago' OUT="api-logs-${STAGE}-2.json" ./utils/search_aws_logs.sh 'LG:api-request-path'
jq -s 'add' "api-logs-${STAGE}-1.json" "api-logs-${STAGE}-2.json" > "api-logs-${STAGE}.json" && rm -f "api-logs-${STAGE}-1.json" "api-logs-${STAGE}-2.json"
./utils/count_apis.sh "api-logs-${STAGE}.json" > "api-logs-${STAGE}.log" && cat "api-logs-${STAGE}.log"
