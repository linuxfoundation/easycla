#!/bin/bash
# GET /gitlab/oauth/callback
# GitLab OAuth callback handler (authenticated)
# Usage: ./get_gitlab_oauth_callback.sh [code] [state]
# Example: ./get_gitlab_oauth_callback.sh "auth_code_123" "state_456"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_gitlab_oauth_callback.sh [code] [state]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build query parameters if provided
QUERY_PARAMS=""
if [ ! -z "$1" ]; then
  QUERY_PARAMS="code=$1"
fi
if [ ! -z "$2" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&state=$2"
  else
    QUERY_PARAMS="state=$2"
  fi
fi

# Set up curl execution
API="${API_URL}/v4/gitlab/oauth/callback"
if [ ! -z "$QUERY_PARAMS" ]; then
  API="${API}?${QUERY_PARAMS}"
fi

CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh