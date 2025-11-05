#!/bin/bash
# DELETE /project/{projectSFID}/gitlab/organization
# Deleteprojectgitlabgroupconfig (authenticated)
# Example: ./delete_project_gitlab_group_config.sh param1
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./delete_project_gitlab_group_config.sh <project_sfid>

if [ -z "$1" ]
then
  echo "$0: you need to specify project_sfid as parameter"
  echo "Usage: $0 <project_sfid>"
  echo "Example: $0 param1"
  exit 1
fi

export project_sfid="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/project/${project_sfid}/gitlab/organization"
CURL_CMD="curl -s -XDELETE -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
