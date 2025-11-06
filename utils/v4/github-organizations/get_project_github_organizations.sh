#!/bin/bash
# Get GitHub organizations associated with a project (authenticated)
# Usage: ./get_project_github_organizations.sh <project_sfid>
# Example: ./get_project_github_organizations.sh a01234567890123456
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_project_github_organizations.sh <project_sfid>

if [ -z "$1" ]
then
  echo "$0: you need to specify project_sfid as a parameter"
  echo "Usage: $0 <project_sfid>"
  echo "Example: $0 a01234567890123456"
  exit 1
fi

export project_sfid="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/project/${project_sfid}/github/organizations"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh