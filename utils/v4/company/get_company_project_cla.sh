#!/bin/bash
# GET /company/{companySFID}/project/{projectSFID}/cla
# Returns the CLA Groups associated with the Project and Company (authenticated)
# Usage: ./get_company_project_cla.sh <company_sfid> <project_sfid>
# Example: ./get_company_project_cla.sh a01234567890123456 a09876543210987654
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_company_project_cla.sh <company_sfid> <project_sfid>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify company_sfid and project_sfid as parameters"
  echo "Usage: $0 <company_sfid> <project_sfid>"
  echo "Example: $0 a01234567890123456 a09876543210987654"
  exit 1
fi

export company_sfid="$1"
export project_sfid="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/company/${company_sfid}/project/${project_sfid}/cla"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh