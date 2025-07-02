#!/bin/bash
# STAGE=dev ./utils/lookup_user_by_github_username.sh lukaszgryglicki
if [ -z "$1" ]
then
  echo "$0: you need to specify GitHub login as a 1st parameter, for example: lukaszgryglicki"
  exit 1
fi
if [ -z "${STAGE}" ]
then
  export STAGE=dev
fi

if [ ! -z "${DEBUG}" ]
then
  echo "aws --profile \"lfproduct-${STAGE}\" dynamodb query --table-name \"cla-${STAGE}-users\" --index-name github-username-index --key-condition-expression \"user_github_username = :github_username\" --expression-attribute-values '{\":github_username\":{\"S\":\"${1}\"}}' | jq -r '.'"
fi
aws --profile "lfproduct-${STAGE}" dynamodb query --table-name "cla-${STAGE}-users" --index-name github-username-index --key-condition-expression "user_github_username = :github_username" --expression-attribute-values "{\":github_username\":{\"S\":\"${1}\"}}" | jq -r '.'
