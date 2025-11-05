#!/bin/bash
# Get company by name (authenticated)
# Usage: ./get_company_by_name.sh "<company_name>"
# Example: ./get_company_by_name.sh "Acme Corporation"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_company_by_name.sh "<company_name>"

if [ -z "$1" ]
then
  echo "$0: you need to specify company_name as a parameter"
  echo "Usage: $0 \"<company_name>\""
  echo "Example: $0 \"Acme Corporation\""
  exit 1
fi

export company_name="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# URL encode the company name
encoded_name=$(echo "$company_name" | sed 's/ /%20/g')

# Set up curl execution
API="${API_URL}/v4/company/name/${encoded_name}"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh