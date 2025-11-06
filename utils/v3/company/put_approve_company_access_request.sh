#!/bin/bash
# PUT /company/{companyID}/cla/accesslist/{requestID}/approve
# Approvecompanyaccessrequest (authenticated)
# Example: ./put_approve_company_access_request.sh param1 param2
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./put_approve_company_access_request.sh <company_id> <request_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify company_id, request_id as parameters"
  echo "Usage: $0 <company_id> <request_id>"
  echo "Example: $0 param1 param2"
  exit 1
fi

export company_id="$1"
export request_id="$2"

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
API="${API_URL}/v3/company/${company_id}/cla/accesslist/${request_id}/approve"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
