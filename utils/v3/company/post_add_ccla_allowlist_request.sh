#!/bin/bash
# POST /company/{companyID}/ccla-whitelist-requests/{projectID}
# Addcclaallowlistrequest (authenticated)
# Example: ./post_add_ccla_allowlist_request.sh param1 param2
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_add_ccla_allowlist_request.sh <company_id> <project_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify company_id, project_id as parameters"
  echo "Usage: $0 <company_id> <project_id>"
  echo "Example: $0 param1 param2"
  exit 1
fi

export company_id="$1"
export project_id="$2"

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
API="${API_URL}/v3/company/${company_id}/ccla-whitelist-requests/${project_id}"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
