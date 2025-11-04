#!/bin/bash
# POST /gitlab/activity
# GitLab activity webhook handler (authenticated)
# Usage: ./post_gitlab_activity.sh <event_type> <action> [payload_file]
# Example: ./post_gitlab_activity.sh "merge_request" "opened" ./sample_payload.json
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_gitlab_activity.sh <event> <action> [payload]

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify event_type and action as parameters"
  echo "Usage: $0 <event_type> <action> [payload_file]"
  echo "Example: $0 \"merge_request\" \"opened\" ./sample_payload.json"
  exit 1
fi

export event_type="$1"
export action="$2"
export payload_file="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
if [ ! -z "$payload_file" ] && [ -f "$payload_file" ]; then
  PAYLOAD=$(cat "$payload_file")
else
  PAYLOAD=$(cat <<EOF
{
  "object_kind": "${event_type}",
  "object_attributes": {
    "action": "${action}"
  }
}
EOF
)
fi

# Set up curl execution
API="${API_URL}/v4/gitlab/activity"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -H \"X-Gitlab-Event: ${event_type}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh