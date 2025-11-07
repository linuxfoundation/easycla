#!/bin/bash
# DELETE /company/sfid/{companySFID}
# Delete company by SFID (authenticated)
# Usage: ./delete_company_by_sfid.sh <company_sfid>
# Example: ./delete_company_by_sfid.sh a09P000000DsNH2IAN
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./delete_company_by_sfid.sh <company_sfid>

if [ -z "$1" ]
then
  echo "$0: you need to specify company_sfid as a parameter"
  echo "Usage: $0 <company_sfid>"
  echo "Example: $0 a09P000000DsNH2IAN"
  exit 1
fi

export company_sfid="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/company/sfid/${company_sfid}"
CURL_CMD="curl -s -XDELETE -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=false
. ./utils/shared/handle_curl_execution.sh