#!/bin/bash
# POST /signed/gerrit/individual/{user_id}
# Submit signed Gerrit individual CLA (authenticated)
# Usage: ./post_signed_gerrit_individual.sh <user_id> <signature_data>
# Example: ./post_signed_gerrit_individual.sh user123 "signature_data_here"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_signed_gerrit_individual.sh <user> <data>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify user_id and signature_data as parameters"
  echo "Usage: $0 <user_id> <signature_data>"
  echo "Example: $0 user123 \"signature_data_here\""
  exit 1
fi

export user_id="$1"
export signature_data="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "signature_data": "${signature_data}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/signed/gerrit/individual/${user_id}"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh