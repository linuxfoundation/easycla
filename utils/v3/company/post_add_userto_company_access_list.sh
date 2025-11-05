#!/bin/bash
# POST /company/{companyID}/cla/accesslist
# Addusertocompanyaccesslist (authenticated)
# Example: ./post_add_userto_company_access_list.sh param1
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_add_userto_company_access_list.sh <company_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify company_id as parameter"
  echo "Usage: $0 <company_id>"
  echo "Example: $0 param1"
  exit 1
fi

export company_id="$1"

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
API="${API_URL}/v3/company/${company_id}/cla/accesslist"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
