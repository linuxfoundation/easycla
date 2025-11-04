#!/bin/bash
# Add CLA Manager to specified Company and Project (authenticated)
# Usage: ./add_cla_manager.sh <company_id> <project_sfid> <first_name> <last_name> <user_email>
# Example: ./add_cla_manager.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a01234567890123456 John Doe john.doe@example.com
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./add_cla_manager.sh <company_id> <project_sfid> <first_name> <last_name> <user_email>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ]
then
  echo "$0: you need to specify company_id, project_sfid, first_name, last_name, and user_email as parameters"
  echo "Usage: $0 <company_id> <project_sfid> <first_name> <last_name> <user_email>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a01234567890123456 John Doe john.doe@example.com"
  exit 1
fi

export company_id="$1"
export project_sfid="$2"
export first_name="$3"
export last_name="$4"
export user_email="$5"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "firstName": "${first_name}",
  "lastName": "${last_name}",
  "userEmail": "${user_email}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/company/${company_id}/project/${project_sfid}/cla-manager"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh