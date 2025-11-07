#!/bin/bash
# GET /company/{companyID}/project/{projectSFID}/cla-managers
# Get CLA manager of company for particular project/foundation (authenticated)
# Usage: ./get_company_project_cla_managers.sh <company_id> <project_sfid>
# Example: ./get_company_project_cla_managers.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a01234567890123456
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_company_project_cla_managers.sh <company_id> <project_sfid>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify company_id and project_sfid as parameters"
  echo "Usage: $0 <company_id> <project_sfid>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a01234567890123456"
  exit 1
fi

export company_id="$1"
export project_sfid="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/company/${company_id}/project/${project_sfid}/cla-managers"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh