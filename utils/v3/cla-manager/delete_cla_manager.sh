#!/bin/bash
# DELETE /company/{companyID}/project/{projectID}/cla-manager/{userLFID}
# Deleteclamanager (authenticated)
# Example: ./delete_cla_manager.sh param1 param2 param3
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./delete_cla_manager.sh <company_id> <project_id> <user_lfid>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify company_id, project_id, user_lfid as parameters"
  echo "Usage: $0 <company_id> <project_id> <user_lfid>"
  echo "Example: $0 param1 param2 param3"
  exit 1
fi

export company_id="$1"
export project_id="$2"
export user_lfid="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v3/company/${company_id}/project/${project_id}/cla-manager/${user_lfid}"
CURL_CMD="curl -s -XDELETE -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
