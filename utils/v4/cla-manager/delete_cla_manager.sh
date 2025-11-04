#!/bin/bash
# Delete CLA Manager from specified Company and Project (authenticated)
# Usage: ./delete_cla_manager.sh <company_id> <project_sfid> <user_lfid>
# Example: ./delete_cla_manager.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a01234567890123456 john.doe
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./delete_cla_manager.sh <company_id> <project_sfid> <user_lfid>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify company_id, project_sfid, and user_lfid as parameters"
  echo "Usage: $0 <company_id> <project_sfid> <user_lfid>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a01234567890123456 john.doe"
  exit 1
fi

export company_id="$1"
export project_sfid="$2"
export user_lfid="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/company/${company_id}/project/${project_sfid}/cla-manager/${user_lfid}"
CURL_CMD="curl -s -XDELETE -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=false
. ./utils/shared/handle_curl_execution.sh