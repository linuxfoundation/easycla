#!/bin/bash
# POST /signed/gitlab/individual/{user_id}/{organization_id}/{gitlab_repository_id}/{merge_request_id}
# Iclacallbackgitlab (authenticated)
# Example: ./post_icla_callback_gitlab.sh param1 param2 param3 param4
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_icla_callback_gitlab.sh <user_id> <organization_id> <gitlab_repository_id> <merge_request_id>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ]
then
  echo "$0: you need to specify user_id, organization_id, gitlab_repository_id, merge_request_id as parameters"
  echo "Usage: $0 <user_id> <organization_id> <gitlab_repository_id> <merge_request_id>"
  echo "Example: $0 param1 param2 param3 param4"
  exit 1
fi

export user_id="$1"
export organization_id="$2"
export gitlab_repository_id="$3"
export merge_request_id="$4"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "example": "value"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/signed/gitlab/individual/${user_id}/${organization_id}/${gitlab_repository_id}/${merge_request_id}"
CURL_CMD="curl -s -XPOST -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
