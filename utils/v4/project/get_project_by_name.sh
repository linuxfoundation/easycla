#!/bin/bash
# Get project by name (authenticated)
# Usage: ./get_project_by_name.sh <project_name>
# Example: ./get_project_by_name.sh "My Project Name"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_project_by_name.sh <project_name>

if [ -z "$1" ]
then
  echo "$0: you need to specify project_name as a parameter"
  echo "Usage: $0 <project_name>"
  echo "Example: $0 \"My Project Name\""
  exit 1
fi

export project_name="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# URL encode the project name
encoded_name=$(printf '%s' "$project_name" | sed 's/ /%20/g' | sed 's/+/%2B/g' | sed 's/&/%26/g')

# Set up curl execution
API="${API_URL}/v4/project/name/${encoded_name}"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh