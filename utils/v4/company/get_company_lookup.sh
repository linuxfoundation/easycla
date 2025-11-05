#!/bin/bash
# GET /company/lookup
# Search companies from organization service (authenticated)
# Usage: ./get_company_lookup.sh [company_name] [website_name]
# Example: ./get_company_lookup.sh "Acme Corp" OR ./get_company_lookup.sh "" "https://acme.com"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_company_lookup.sh [company_name] [website]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build query parameters
QUERY_PARAMS=""
if [ ! -z "$1" ]; then
  QUERY_PARAMS="companyName=$(echo "$1" | sed 's/ /%20/g')"
fi

if [ ! -z "$2" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&websiteName=$(echo "$2" | sed 's/:/%3A/g; s/\//%2F/g')"
  else
    QUERY_PARAMS="websiteName=$(echo "$2" | sed 's/:/%3A/g; s/\//%2F/g')"
  fi
fi

if [ -z "$QUERY_PARAMS" ]; then
  echo "$0: you need to specify either company_name or website_name as parameters"
  echo "Usage: $0 [company_name] [website_name]"  
  echo "Example: $0 \"Acme Corp\" OR $0 \"\" \"https://acme.com\""
  exit 1
fi

# Set up curl execution
API="${API_URL}/v4/company/lookup?${QUERY_PARAMS}"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh