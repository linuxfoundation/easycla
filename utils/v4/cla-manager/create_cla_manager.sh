#!/bin/bash
# POST /company/{companyID}/project/{projectSFID}/cla-manager
# Createclamanager (authenticated)
# Example: ./create_cla_manager.sh param1 param2
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./create_cla_manager.sh <company_id> <project_sfid>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify company_id, project_sfid as parameters"
  echo "Usage: $0 <company_id> <project_sfid>"
  echo "Example: $0 param1 param2"
  exit 1
fi

export company_id="$1"
export project_sfid="$2"

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
API="${API_URL}/v4/company/${company_id}/project/${project_sfid}/cla-manager"
CURL_CMD="curl -s -XPOST -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
