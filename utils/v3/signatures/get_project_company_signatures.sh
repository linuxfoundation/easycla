#!/bin/bash
# GET /signatures/project/{projectID}/company/{companyID}
# Getprojectcompanysignatures (authenticated)
# Example: ./get_project_company_signatures.sh param1 param2
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_project_company_signatures.sh <project_id> <company_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify project_id, company_id as parameters"
  echo "Usage: $0 <project_id> <company_id>"
  echo "Example: $0 param1 param2"
  exit 1
fi

export project_id="$1"
export company_id="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v3/signatures/project/${project_id}/company/${company_id}"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
