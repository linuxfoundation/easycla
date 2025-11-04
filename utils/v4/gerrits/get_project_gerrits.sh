#!/bin/bash
# GET /cla-group/{claGroupID}/project/{projectSFID}/gerrits
# Get Gerrit repositories for a project in a CLA group (authenticated)
# Usage: ./get_project_gerrits.sh <cla_group_id> <project_sfid>
# Example: ./get_project_gerrits.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a09P000000DsNH2IAN
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_project_gerrits.sh <cla_group_id> <project_sfid>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify cla_group_id and project_sfid as parameters"
  echo "Usage: $0 <cla_group_id> <project_sfid>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a09P000000DsNH2IAN"
  exit 1
fi

export cla_group_id="$1"
export project_sfid="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/cla-group/${cla_group_id}/project/${project_sfid}/gerrits"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh