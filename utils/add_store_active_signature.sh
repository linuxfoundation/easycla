#!/bin/bash
# repo: aws --region us-east-1 --profile lfproduct-dev dynamodb scan --table-name "cla-dev-repositories"
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
if [ -z "${1}" ]
then
  echo "$0: you must specify user ID value as a 1st argument, like: 'b817eb57-045a-4fe0-8473-fbb416a01d70'"
  exit 1
fi
if [ -z "${2}" ]
then
  echo "$0: you must specify project ID value as a 2nd argument, like: 'd8cead54-92b7-48c5-a2c8-b1e295e8f7f1'"
  exit 2
fi
EXPIRE_TS=$(date -d 'tomorrow' +%s)
aws --profile "lfproduct-${STAGE}" dynamodb put-item --table-name "cla-${STAGE}-store" --item '{
    "key": {
      "S": "active_signature:'"${1}"'"
    },
    "value": {
      "S": "{\"user_id\": \"'"${1}"'\", \"project_id\": \"'"${2}"'\", \"repository_id\": \"466156917\", \"pull_request_id\": \"3\"}"
    },
    "expire": {
      "N": "'"${EXPIRE_TS}"'"
    }
  }'
