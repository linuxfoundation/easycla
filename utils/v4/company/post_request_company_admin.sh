#!/bin/bash
# POST /user/{userID}/request-company-admin
# Request Company Admin based on user request to sign CLA (authenticated)
# Usage: ./post_request_company_admin.sh <user_id> <cla_manager_email> <cla_manager_name> <company_name>
# Example: ./post_request_company_admin.sh user123 "admin@company.com" "John Admin" "Acme Corp"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_request_company_admin.sh <user_id> <email> <name> <company>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ]
then
  echo "$0: you need to specify user_id, cla_manager_email, cla_manager_name, and company_name as parameters"
  echo "Usage: $0 <user_id> <cla_manager_email> <cla_manager_name> <company_name>"
  echo "Example: $0 user123 \"admin@company.com\" \"John Admin\" \"Acme Corp\""
  exit 1
fi

export user_id="$1"
export cla_manager_email="$2"
export cla_manager_name="$3"
export company_name="$4"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "claManagerEmail": "${cla_manager_email}",
  "claManagerName": "${cla_manager_name}",
  "companyName": "${company_name}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/user/${user_id}/request-company-admin"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh