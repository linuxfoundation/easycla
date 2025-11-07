#!/bin/bash
# GET /project/name/{projectName}
# Getprojectbyname (authenticated)
# Example: ./get_project_by_name.sh param1
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_project_by_name.sh <project_name>

if [ -z "$1" ]
then
  echo "$0: you need to specify project_name as parameter"
  echo "Usage: $0 <project_name>"
  echo "Example: $0 param1"
  exit 1
fi

export project_name="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v3/project/name/${project_name}"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
