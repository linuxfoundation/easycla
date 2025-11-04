#!/bin/bash
# PUT /cla-group/{claGroupID}/user/{userID}/icla
# Update user ICLA signature (authenticated)
# Usage: ./put_icla_signature.sh <cla_group_id> <user_id> <signature_data>
# Example: ./put_icla_signature.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e user123 "signature_data"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./put_icla_signature.sh <cla_group_id> <user_id> <data>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify cla_group_id, user_id, and signature_data as parameters"
  echo "Usage: $0 <cla_group_id> <user_id> <signature_data>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e user123 \"signature_data\""
  exit 1
fi

export cla_group_id="$1"
export user_id="$2"
export signature_data="$3"

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
API="${API_URL}/v4/cla-group/${cla_group_id}/user/${user_id}/icla"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh