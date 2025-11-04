#!/bin/bash
# Get project company employee signatures (authenticated)
# Usage: ./get_project_company_employee_signatures.sh <project_sfid> <company_id>
# Example: ./get_project_company_employee_signatures.sh a01234567890123456 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_project_company_employee_signatures.sh <project_sfid> <company_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify project_sfid and company_id as parameters"
  echo "Usage: $0 <project_sfid> <company_id>"
  echo "Example: $0 a01234567890123456 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e"
  exit 1
fi

export project_sfid="$1"
export company_id="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/signatures/project/${project_sfid}/company/${company_id}/employee"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh