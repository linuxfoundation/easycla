#!/bin/bash
# POST /user/{userID}/company
# Createcompany (authenticated)
# Example: ./create_company.sh param1
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./create_company.sh <user_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify user_id as parameter"
  echo "Usage: $0 <user_id>"
  echo "Example: $0 param1"
  exit 1
fi

export user_id="$1"

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
API="${API_URL}/v4/user/${user_id}/company"
CURL_CMD="curl -s -XPOST -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
