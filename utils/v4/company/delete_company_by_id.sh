#!/bin/bash
# DELETE /company/id/{companyID}
# Delete company by ID (authenticated)
# Usage: ./delete_company_by_id.sh <company_id>
# Example: ./delete_company_by_id.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./delete_company_by_id.sh <company_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify company_id as a parameter"
  echo "Usage: $0 <company_id>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e"
  exit 1
fi

export company_id="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/company/id/${company_id}"
CURL_CMD="curl -s -XDELETE -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=false
. ./utils/shared/handle_curl_execution.sh