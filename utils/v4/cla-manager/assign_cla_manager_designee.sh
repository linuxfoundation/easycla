#!/bin/bash
# Assign CLA Manager Designee to a CLA Group/Company (authenticated)
# Usage: ./assign_cla_manager_designee.sh <company_id> <cla_group_id> <user_email>
# Example: ./assign_cla_manager_designee.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e e1234567-abcd-4567-8901-234567890abc john.doe@example.com
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./assign_cla_manager_designee.sh <company_id> <cla_group_id> <user_email>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify company_id, cla_group_id, and user_email as parameters"
  echo "Usage: $0 <company_id> <cla_group_id> <user_email>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e e1234567-abcd-4567-8901-234567890abc john.doe@example.com"
  exit 1
fi

export company_id="$1"
export cla_group_id="$2"
export user_email="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "userEmail": "${user_email}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/company/${company_id}/claGroup/${cla_group_id}/cla-manager-designee"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh