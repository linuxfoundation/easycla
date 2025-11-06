#!/bin/bash
# PUT /company/{companyID}/ccla-whitelist-requests/{projectID}/{requestID}/approve
# Approvecclaallowlistrequest (authenticated)
# Example: ./put_approve_ccla_allowlist_request.sh param1 param2 param3
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./put_approve_ccla_allowlist_request.sh <company_id> <project_id> <request_id>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify company_id, project_id, request_id as parameters"
  echo "Usage: $0 <company_id> <project_id> <request_id>"
  echo "Example: $0 param1 param2 param3"
  exit 1
fi

export company_id="$1"
export project_id="$2"
export request_id="$3"

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
API="${API_URL}/v3/company/${company_id}/ccla-whitelist-requests/${project_id}/${request_id}/approve"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
