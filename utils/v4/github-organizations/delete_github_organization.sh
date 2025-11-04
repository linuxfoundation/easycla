#!/bin/bash
# DELETE /project/{projectSFID}/github/organizations/{orgName}
# Remove GitHub organization from project (authenticated)
# Usage: ./delete_github_organization.sh <project_sfid> <org_name>
# Example: ./delete_github_organization.sh a01234567890123456 myorg
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./delete_github_organization.sh <project_sfid> <org_name>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify project_sfid and org_name as parameters"
  echo "Usage: $0 <project_sfid> <org_name>"
  echo "Example: $0 a01234567890123456 myorg"
  exit 1
fi

export project_sfid="$1"
export org_name="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/project/${project_sfid}/github/organizations/${org_name}"
CURL_CMD="curl -s -XDELETE -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=false
. ./utils/shared/handle_curl_execution.sh