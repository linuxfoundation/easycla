#!/bin/bash
# Update project (authenticated)
# Usage: ./update_project.sh <project_name> <project_description>
# Example: ./update_project.sh "My Updated Project" "Updated project description"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./update_project.sh <project_name> <description>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify project_name and project_description as parameters"
  echo "Usage: $0 <project_name> <project_description>"
  echo "Example: $0 \"My Updated Project\" \"Updated project description\""
  exit 1
fi

export project_name="$1"
export project_description="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "project_name": "${project_name}",
  "project_description": "${project_description}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/project"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh