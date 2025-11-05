#!/bin/bash
# GET /project/{projectSFID}/github/repositories/{repositoryID}/branch-protection
# Get branch protection status for GitHub repository (authenticated)
# Usage: ./get_branch_protection.sh <project_sfid> <repository_id>
# Example: ./get_branch_protection.sh a09P000000DsNH2IAN repo123
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_branch_protection.sh <project_sfid> <repository_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify project_sfid and repository_id as parameters"
  echo "Usage: $0 <project_sfid> <repository_id>"
  echo "Example: $0 a09P000000DsNH2IAN repo123"
  exit 1
fi

export project_sfid="$1"
export repository_id="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/project/${project_sfid}/github/repositories/${repository_id}/branch-protection"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh