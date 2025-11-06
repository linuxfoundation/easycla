#!/bin/bash
# Get foundation mapping (CLA groups under foundation) (authenticated)
# Usage: ./get_foundation_mapping.sh [foundation_sfid]
# Example: ./get_foundation_mapping.sh a01234567890123456
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_foundation_mapping.sh [foundation_sfid]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build query parameters
QUERY_PARAMS=""
if [ ! -z "$1" ]; then
  QUERY_PARAMS="foundationSFID=$1"
fi

# Set up curl execution
API="${API_URL}/v4/foundation-mapping"
if [ ! -z "$QUERY_PARAMS" ]; then
  API="${API}?${QUERY_PARAMS}"
fi

CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh