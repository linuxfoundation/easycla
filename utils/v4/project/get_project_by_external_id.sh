#!/bin/bash
# Get project by external ID (SFID) (authenticated)
# Usage: ./get_project_by_external_id.sh <external_id>
# Example: ./get_project_by_external_id.sh a01234567890123456
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_project_by_external_id.sh <external_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify external_id as a parameter"
  echo "Usage: $0 <external_id>"
  echo "Example: $0 a01234567890123456"
  exit 1
fi

export external_id="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/project/external/${external_id}"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh