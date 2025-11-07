#!/bin/bash
# PUT /cla-group/{claGroupID}/user/{userID}/icla
# Invalidateicla (authenticated)
# Example: ./put_invalidate_icla.sh param1 param2
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./put_invalidate_icla.sh <cla_group_id> <user_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify cla_group_id, user_id as parameters"
  echo "Usage: $0 <cla_group_id> <user_id>"
  echo "Example: $0 param1 param2"
  exit 1
fi

export cla_group_id="$1"
export user_id="$2"

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
API="${API_URL}/v4/cla-group/${cla_group_id}/user/${user_id}/icla"
CURL_CMD="curl -s -XPUT -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
