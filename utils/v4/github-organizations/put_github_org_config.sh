#!/bin/bash
# PUT /project/{projectSFID}/github/organizations/{orgName}/config
# Update GitHub Organization Configuration (authenticated)
# Usage: ./put_github_org_config.sh <project_sfid> <org_name> <auto_enabled> <cla_group_id> <branch_protection>
# Example: ./put_github_org_config.sh a01234567890123456 myorg true d9428888-122b-4b20-8c4a-0c9a1a6f9b8e true
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./put_github_org_config.sh <project_sfid> <org_name> <auto> <cla_group> <protection>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ]
then
  echo "$0: you need to specify project_sfid, org_name, auto_enabled, cla_group_id, and branch_protection as parameters"
  echo "Usage: $0 <project_sfid> <org_name> <auto_enabled> <cla_group_id> <branch_protection>"
  echo "Example: $0 a01234567890123456 myorg true d9428888-122b-4b20-8c4a-0c9a1a6f9b8e true"
  exit 1
fi

export project_sfid="$1"
export org_name="$2" 
export auto_enabled="$3"
export cla_group_id="$4"
export branch_protection="$5"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "autoEnabled": ${auto_enabled},
  "autoEnabledClaGroupID": "${cla_group_id}",
  "branchProtectionEnabled": ${branch_protection}
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/project/${project_sfid}/github/organizations/${org_name}/config"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh