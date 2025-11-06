#!/bin/bash
# DELETE /project/{projectSFID}/github/repositories/{repositoryID}
# Remove GitHub repository from project (authenticated)
# Usage: ./delete_github_repository.sh <project_sfid> <repository_id>
# Example: ./delete_github_repository.sh a09P000000DsNH2IAN repo123
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./delete_github_repository.sh <project_sfid> <repository_id>

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
API="${API_URL}/v4/project/${project_sfid}/github/repositories/${repository_id}"
CURL_CMD="curl -s -XDELETE -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=false
. ./utils/shared/handle_curl_execution.sh