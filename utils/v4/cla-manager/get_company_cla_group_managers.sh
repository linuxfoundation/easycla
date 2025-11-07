#!/bin/bash
# GET /company/{companyID}/cla-group/{claGroupID}/cla-managers
# Get list of CLA managers based on the CLA Group and v1 Company ID (authenticated)
# Usage: ./get_company_cla_group_managers.sh <company_id> <cla_group_id>
# Example: ./get_company_cla_group_managers.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a12345678-1234-1234-1234-123456789012
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_company_cla_group_managers.sh <company_id> <cla_group_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify company_id and cla_group_id as parameters"
  echo "Usage: $0 <company_id> <cla_group_id>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a12345678-1234-1234-1234-123456789012"
  exit 1
fi

export company_id="$1"
export cla_group_id="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/company/${company_id}/cla-group/${cla_group_id}/cla-managers"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh