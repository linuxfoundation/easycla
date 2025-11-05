#!/bin/bash
# Get recent events (authenticated)
# Usage: ./get_recent_events.sh [pageSize] [nextKey]
# Example: ./get_recent_events.sh 50
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_recent_events.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build query parameters
QUERY_PARAMS=""
if [ ! -z "$1" ]; then
  QUERY_PARAMS="pageSize=$1"
fi

if [ ! -z "$2" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&nextKey=$(echo "$2" | sed 's/ /%20/g')"
  else
    QUERY_PARAMS="nextKey=$(echo "$2" | sed 's/ /%20/g')"
  fi
fi

# Set up curl execution
API="${API_URL}/v4/events/recent"
if [ ! -z "$QUERY_PARAMS" ]; then
  API="${API}?${QUERY_PARAMS}"
fi

CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh