#!/bin/bash
# DELETE /project/{projectID}/gerrits/{gerritID}
# Deletegerrit (authenticated)
# Example: ./delete_gerrit.sh param1 param2
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./delete_gerrit.sh <project_id> <gerrit_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify project_id, gerrit_id as parameters"
  echo "Usage: $0 <project_id> <gerrit_id>"
  echo "Example: $0 param1 param2"
  exit 1
fi

export project_id="$1"
export gerrit_id="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v3/project/${project_id}/gerrits/${gerrit_id}"
CURL_CMD="curl -s -XDELETE -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
