#!/bin/bash
# Get GitLab group members (public endpoint)
# Usage: ./get_gitlab_group_members.sh <gitlab_group_id>
# Example: ./get_gitlab_group_members.sh 12345

if [ -z "$1" ]
then
  echo "$0: you need to specify gitlab_group_id as a parameter"
  echo "Usage: $0 <gitlab_group_id>"
  echo "Example: $0 12345"
  exit 1
fi

export gitlab_group_id="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/gitlab/group/${gitlab_group_id}/members"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh