#!/bin/bash
# List the metrics for projects (authenticated)
# Usage: ./list_project_metrics.sh [nextKey] [pageSize]
# Example: ./list_project_metrics.sh
# Example: ./list_project_metrics.sh "next-key-value" 50
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./list_project_metrics.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build query parameters
QUERY_PARAMS=""
if [ ! -z "$1" ]; then
  QUERY_PARAMS="nextKey=$(echo "$1" | sed 's/ /%20/g')"
fi

if [ ! -z "$2" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&pageSize=$2"
  else
    QUERY_PARAMS="pageSize=$2"
  fi
fi

# Set up curl execution
API="${API_URL}/v4/metrics/project"
if [ ! -z "$QUERY_PARAMS" ]; then
  API="${API}?${QUERY_PARAMS}"
fi

CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh