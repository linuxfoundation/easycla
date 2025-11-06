#!/bin/bash
# PUT /signatures/project/{projectID}/company/{companyID}/clagroup/{claGroupID}/approval-list
# Updateapprovallist (authenticated)
# Example: ./update_approval_list.sh param1 param2 param3
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./update_approval_list.sh <project_id> <company_id> <cla_group_id>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify project_id, company_id, cla_group_id as parameters"
  echo "Usage: $0 <project_id> <company_id> <cla_group_id>"
  echo "Example: $0 param1 param2 param3"
  exit 1
fi

export project_id="$1"
export company_id="$2"
export cla_group_id="$3"

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
API="${API_URL}/v3/signatures/project/${project_id}/company/${company_id}/clagroup/${cla_group_id}/approval-list"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
