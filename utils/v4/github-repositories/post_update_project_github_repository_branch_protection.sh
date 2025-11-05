#!/bin/bash
# POST /project/{projectSFID}/github/repositories/{repositoryID}/branch-protection
# Updateprojectgithubrepositorybranchprotection (authenticated)
# Example: ./post_update_project_github_repository_branch_protection.sh param1 param2
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_update_project_github_repository_branch_protection.sh <project_sfid> <repository_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify project_sfid, repository_id as parameters"
  echo "Usage: $0 <project_sfid> <repository_id>"
  echo "Example: $0 param1 param2"
  exit 1
fi

export project_sfid="$1"
export repository_id="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "example": "value"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/project/${project_sfid}/github/repositories/${repository_id}/branch-protection"
CURL_CMD="curl -s -XPOST -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
