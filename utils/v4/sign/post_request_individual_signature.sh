#!/bin/bash
# POST /request-individual-signature
# Request individual signature (authenticated)  
# Usage: ./post_request_individual_signature.sh <project_id> <user_id> <return_url>
# Example: ./post_request_individual_signature.sh proj123 user456 "https://example.com/return"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_request_individual_signature.sh <proj> <user> <url>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify project_id, user_id, and return_url as parameters"
  echo "Usage: $0 <project_id> <user_id> <return_url>"
  echo "Example: $0 proj123 user456 \"https://example.com/return\""
  exit 1
fi

export project_id="$1"
export user_id="$2"
export return_url="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "project_id": "${project_id}",
  "user_id": "${user_id}",
  "return_url": "${return_url}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/request-individual-signature"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh