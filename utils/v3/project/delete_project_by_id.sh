#!/bin/bash
# DELETE /project/{projectID}
# Deleteprojectbyid (authenticated)
# Example: ./delete_project_by_id.sh param1
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./delete_project_by_id.sh <project_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify project_id as parameter"
  echo "Usage: $0 <project_id>"
  echo "Example: $0 param1"
  exit 1
fi

export project_id="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v3/project/${project_id}"
CURL_CMD="curl -s -XDELETE -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
