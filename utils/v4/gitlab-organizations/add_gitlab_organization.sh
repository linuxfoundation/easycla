#!/bin/bash
# Add GitLab organization to project (authenticated)
# Usage: ./add_gitlab_organization.sh <project_sfid> <group_id> <org_full_path> <auto_enabled> <cla_group_id> <branch_protection>
# Example: ./add_gitlab_organization.sh a09P000000DsNH2IAN 12345 "myorg/myproject" false d9428888-122b-4b20-8c4a-0c9a1a6f9b8e false
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./add_gitlab_organization.sh <params>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ] || [ -z "$6" ]
then
  echo "$0: you need to specify project_sfid, group_id, org_full_path, auto_enabled, cla_group_id, and branch_protection as parameters"
  echo "Usage: $0 <project_sfid> <group_id> <org_full_path> <auto_enabled> <cla_group_id> <branch_protection>"
  echo "Example: $0 a09P000000DsNH2IAN 12345 \"myorg/myproject\" false d9428888-122b-4b20-8c4a-0c9a1a6f9b8e false"
  exit 1
fi

export project_sfid="$1"
export group_id="$2"
export org_full_path="$3"
export auto_enabled="$4"
export cla_group_id="$5"
export branch_protection="$6"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "auto_enabled": ${auto_enabled},
  "auto_enabled_cla_group_id": "${cla_group_id}",
  "branch_protection_enabled": ${branch_protection},
  "group_id": ${group_id},
  "organization_full_path": "${org_full_path}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/project/${project_sfid}/gitlab/organizations"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh