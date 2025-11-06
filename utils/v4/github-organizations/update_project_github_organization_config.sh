#!/bin/bash
# PUT /project/{projectSFID}/github/organizations/{orgName}/config
# Updateprojectgithuborganizationconfig (authenticated)
# Example: ./update_project_github_organization_config.sh param1 param2
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./update_project_github_organization_config.sh <project_sfid> <org_name>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify project_sfid, org_name as parameters"
  echo "Usage: $0 <project_sfid> <org_name>"
  echo "Example: $0 param1 param2"
  exit 1
fi

export project_sfid="$1"
export org_name="$2"

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
API="${API_URL}/v4/project/${project_sfid}/github/organizations/${org_name}/config"
CURL_CMD="curl -s -XPUT -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
