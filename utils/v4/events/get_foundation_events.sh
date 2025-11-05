#!/bin/bash
# Get events for a foundation (authenticated)
# Usage: ./get_foundation_events.sh <foundation_sfid> [pageSize] [nextKey]
# Example: ./get_foundation_events.sh a01234567890123456 50
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_foundation_events.sh <foundation_sfid>

if [ -z "$1" ]
then
  echo "$0: you need to specify foundation_sfid as a parameter"
  echo "Usage: $0 <foundation_sfid> [pageSize] [nextKey]"
  echo "Example: $0 a01234567890123456 50"
  exit 1
fi

export foundation_sfid="$1"

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

if [ ! -z "$3" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&nextKey=$(echo "$3" | sed 's/ /%20/g')"
  else
    QUERY_PARAMS="nextKey=$(echo "$3" | sed 's/ /%20/g')"
  fi
fi

# Set up curl execution
API="${API_URL}/v4/events/foundation/${foundation_sfid}"
if [ ! -z "$QUERY_PARAMS" ]; then
  API="${API}?${QUERY_PARAMS}"
fi

CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh