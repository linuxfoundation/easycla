#!/bin/bash
# Update GitLab group configuration (authenticated)
# Usage: ./update_gitlab_group_config.sh <project_sfid> <gitlab_group_id> <auto_enabled> <cla_group_id> <branch_protection>
# Example: ./update_gitlab_group_config.sh a09P000000DsNH2IAN 12345 true d9428888-122b-4b20-8c4a-0c9a1a6f9b8e true
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./update_gitlab_group_config.sh <params>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ]
then
  echo "$0: you need to specify project_sfid, gitlab_group_id, auto_enabled, cla_group_id, and branch_protection as parameters"
  echo "Usage: $0 <project_sfid> <gitlab_group_id> <auto_enabled> <cla_group_id> <branch_protection>"
  echo "Example: $0 a09P000000DsNH2IAN 12345 true d9428888-122b-4b20-8c4a-0c9a1a6f9b8e true"
  exit 1
fi

export project_sfid="$1"
export gitlab_group_id="$2"
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
  "auto_enabled": ${auto_enabled},
  "auto_enabled_cla_group_id": "${cla_group_id}",
  "branch_protection_enabled": ${branch_protection}
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/project/${project_sfid}/gitlab/group/${gitlab_group_id}/config"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh