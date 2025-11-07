#!/bin/bash
# DELETE /cla-group/{claGroupID}/project/{projectSFID}/gerrits/{gerritID}
# Delete Gerrit repository from project in CLA group (authenticated)
# Usage: ./delete_project_gerrit.sh <cla_group_id> <project_sfid> <gerrit_id>
# Example: ./delete_project_gerrit.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a09P000000DsNH2IAN gerrit123
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./delete_project_gerrit.sh <cla_group_id> <project_sfid> <gerrit_id>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify cla_group_id, project_sfid, and gerrit_id as parameters"
  echo "Usage: $0 <cla_group_id> <project_sfid> <gerrit_id>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a09P000000DsNH2IAN gerrit123"
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
CURL_CMD="curl -s -XDELETE -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=false
. ./utils/shared/handle_curl_execution.sh