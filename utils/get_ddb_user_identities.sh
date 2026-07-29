#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# Returns the identity data (as consumable by my_clas.sh) from every cla-${STAGE}-users record matching the given LF username.
# These are the identities that authorize a My CLAs search from the DynamoDB (EasyCLA record) side.
# STAGE: dev (default) | test | staging | prod. Usage: [STAGE=dev] ./utils/get_ddb_user_identities.sh <lfid>

if [ -z "$1" ]
then
  echo "$0: you need to specify lfid as a 1st parameter, for example: 'lgryglicki'"
  exit 1
fi
if [ -z "$STAGE" ]
then
  export STAGE=dev
fi

cmd=(aws --profile "lfproduct-${STAGE}" dynamodb query --table-name "cla-${STAGE}-users" --index-name lf-username-index --key-condition-expression "lf_username = :name" --expression-attribute-values "{\":name\":{\"S\":\"${1}\"}}")
if [ -n "$DEBUG" ]
then
  echo "${cmd[*]}"
fi
"${cmd[@]}" | jq -r '.Items | map({
  user_id: .user_id.S,
  lfUsername: .lf_username.S,
  email: .lf_email.S,
  secondaryEmail: (.user_emails.SS // []),
  githubId: .user_github_id.N,
  githubUsername: .user_github_username.S,
  gitlabId: .user_gitlab_id.N,
  gitlabUsername: .user_gitlab_username.S
})'
