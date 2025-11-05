#!/bin/bash
# POST /signed/individual/{installation_id}/{github_repository_id}/{change_request_id}
# Iclacallbackgithub (authenticated)
# Example: ./post_icla_callback_github.sh param1 param2 param3
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_icla_callback_github.sh <installation_id> <github_repository_id> <change_request_id>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify installation_id, github_repository_id, change_request_id as parameters"
  echo "Usage: $0 <installation_id> <github_repository_id> <change_request_id>"
  echo "Example: $0 param1 param2 param3"
  exit 1
fi

export installation_id="$1"
export github_repository_id="$2"
export change_request_id="$3"

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
API="${API_URL}/v4/signed/individual/${installation_id}/${github_repository_id}/${change_request_id}"
CURL_CMD="curl -s -XPOST -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
