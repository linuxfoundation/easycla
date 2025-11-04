#!/bin/bash
# Add CLA Manager Request to specified Company and Project (authenticated)
# Usage: ./add_cla_manager_request.sh <company_id> <project_sfid> <full_name> [contact_admin]
# Example: ./add_cla_manager_request.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a01234567890123456 "John Doe" true
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./add_cla_manager_request.sh <company_id> <project_sfid> <full_name>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify company_id, project_sfid, and full_name as parameters"
  echo "Usage: $0 <company_id> <project_sfid> <full_name> [contact_admin]"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a01234567890123456 \"John Doe\" true"
  exit 1
fi

export company_id="$1"
export project_sfid="$2"
export full_name="$3"
export contact_admin="${4:-false}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "fullName": "${full_name}",
  "contactAdmin": ${contact_admin}
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/company/${company_id}/project/${project_sfid}/cla-manager/requests"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh