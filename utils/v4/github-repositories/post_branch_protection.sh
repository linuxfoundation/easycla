#!/bin/bash
# POST /project/{projectSFID}/github/repositories/{repositoryID}/branch-protection
# Enable branch protection for GitHub repository (authenticated)
# Usage: ./post_branch_protection.sh <project_sfid> <repository_id> <branch_name>
# Example: ./post_branch_protection.sh a09P000000DsNH2IAN repo123 main
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_branch_protection.sh <project_sfid> <repository_id> <branch>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify project_sfid, repository_id, and branch_name as parameters"
  echo "Usage: $0 <project_sfid> <repository_id> <branch_name>"
  echo "Example: $0 a09P000000DsNH2IAN repo123 main"
  exit 1
fi

export project_sfid="$1"
export repository_id="$2"
export branch_name="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "branch_name": "${branch_name}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/project/${project_sfid}/github/repositories/${repository_id}/branch-protection"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh