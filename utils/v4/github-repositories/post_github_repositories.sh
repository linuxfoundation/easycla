#!/bin/bash
# POST /project/{projectSFID}/github/repositories
# Add GitHub repository to project (authenticated)
# Usage: ./post_github_repositories.sh <project_sfid> <repository_id> <repository_name> <auto_enabled> <cla_group_id>
# Example: ./post_github_repositories.sh a09P000000DsNH2IAN repo123 "My Repo" true d9428888-122b-4b20-8c4a-0c9a1a6f9b8e
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_github_repositories.sh <params>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ]
then
  echo "$0: you need to specify project_sfid, repository_id, repository_name, auto_enabled, and cla_group_id as parameters"
  echo "Usage: $0 <project_sfid> <repository_id> <repository_name> <auto_enabled> <cla_group_id>"
  echo "Example: $0 a09P000000DsNH2IAN repo123 \"My Repo\" true d9428888-122b-4b20-8c4a-0c9a1a6f9b8e"
  exit 1
fi

export project_sfid="$1"
export repository_id="$2"
export repository_name="$3"
export auto_enabled="$4"
export cla_group_id="$5"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "repository_id": "${repository_id}",
  "repository_name": "${repository_name}",
  "auto_enabled": ${auto_enabled},
  "auto_enabled_cla_group_id": "${cla_group_id}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/project/${project_sfid}/github/repositories"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh