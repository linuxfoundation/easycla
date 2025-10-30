#!/bin/bash
# Search users with optional parameters
# Usage: ./search_users.sh [searchTerm] [fullMatch] [pageSize]
# Example: ./search_users.sh lukasz true 50
# API_URL=http://localhost:5001 TOKEN="$(cat ./token.secret)" ./search_users.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org TOKEN="$(cat ./token.secret)" ./search_users.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build query parameters
QUERY_PARAMS=""
if [ ! -z "$1" ]; then
  QUERY_PARAMS="searchTerm=$1"
fi
if [ ! -z "$2" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&fullMatch=$2"
  else
    QUERY_PARAMS="fullMatch=$2"
  fi
fi
if [ ! -z "$3" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&pageSize=$3"
  else
    QUERY_PARAMS="pageSize=$3"
  fi
fi

API="${API_URL}/v3/users/search"
if [ ! -z "$QUERY_PARAMS" ]; then
  API="${API}?${QUERY_PARAMS}"
fi

# Set up curl execution
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh