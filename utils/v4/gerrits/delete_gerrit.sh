#!/bin/bash
# DELETE /cla-group/{claGroupID}/project/{projectSFID}/gerrits/{gerritID}
# Deletegerrit (authenticated)
# Example: ./delete_gerrit.sh param1 param2 param3
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./delete_gerrit.sh <cla_group_id> <project_sfid> <gerrit_id>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify cla_group_id, project_sfid, gerrit_id as parameters"
  echo "Usage: $0 <cla_group_id> <project_sfid> <gerrit_id>"
  echo "Example: $0 param1 param2 param3"
  exit 1
fi

export cla_group_id="$1"
export project_sfid="$2"
export gerrit_id="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/cla-group/${cla_group_id}/project/${project_sfid}/gerrits/${gerrit_id}"
CURL_CMD="curl -s -XDELETE -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
