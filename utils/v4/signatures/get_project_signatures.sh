#!/bin/bash
# Get project signatures (authenticated)
# Usage: ./get_project_signatures.sh <project_id> [pageSize]
# Example: ./get_project_signatures.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e 10
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_project_signatures.sh <project_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify project_id as a parameter"
  echo "Usage: $0 <project_id> [pageSize]"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e 10"
  exit 1
fi

export project_id="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build query parameters
QUERY_PARAMS=""
if [ ! -z "$2" ]; then
  QUERY_PARAMS="pageSize=$2"
fi

# Set up curl execution
API="${API_URL}/v4/signatures/project/${project_id}"
if [ ! -z "$QUERY_PARAMS" ]; then
  API="${API}?${QUERY_PARAMS}"
fi

CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh