#!/bin/bash
# GET /company/{companyID}/project/{projectID}/cla-manager/requests
# Getclamanagerrequests (authenticated)
# Example: ./get_cla_manager_requests.sh param1 param2
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_cla_manager_requests.sh <company_id> <project_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify company_id, project_id as parameters"
  echo "Usage: $0 <company_id> <project_id>"
  echo "Example: $0 param1 param2"
  exit 1
fi

export company_id="$1"
export project_id="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v3/company/${company_id}/project/${project_id}/cla-manager/requests"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
