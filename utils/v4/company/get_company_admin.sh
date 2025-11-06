#!/bin/bash
# GET /company/{companySFID}/admin
# Returns a list of Company Admins (salesforce) (authenticated)
# Usage: ./get_company_admin.sh <company_sfid>
# Example: ./get_company_admin.sh a01234567890123456
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_company_admin.sh <company_sfid>

if [ -z "$1" ]
then
  echo "$0: you need to specify company_sfid as a parameter"
  echo "Usage: $0 <company_sfid>"
  echo "Example: $0 a01234567890123456"
  exit 1
fi

export company_sfid="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/company/${company_sfid}/admin"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh