#!/bin/bash
# POST /gitlab/trigger
# GitLab trigger handler (authenticated)
# Usage: ./post_gitlab_trigger.sh <trigger_data>
# Example: ./post_gitlab_trigger.sh "trigger_data_123"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_gitlab_trigger.sh <trigger_data>

if [ -z "$1" ]
then
  echo "$0: you need to specify trigger_data as a parameter"
  echo "Usage: $0 <trigger_data>"
  echo "Example: $0 \"trigger_data_123\""
  exit 1
fi

export trigger_data="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "trigger": "${trigger_data}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/gitlab/trigger"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh