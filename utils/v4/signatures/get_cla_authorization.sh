#!/bin/bash
# GET /cla/authorization
# Get CLA authorization for signing (authenticated)
# Usage: ./get_cla_authorization.sh [project_id] [user_id] [company_id]
# Example: ./get_cla_authorization.sh proj123 user456 comp789
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_cla_authorization.sh [proj] [user] [comp]

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build query parameters
QUERY_PARAMS=""
if [ ! -z "$1" ]; then
  QUERY_PARAMS="projectID=$1"
fi

if [ ! -z "$2" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&userID=$2"
  else
    QUERY_PARAMS="userID=$2"
  fi
fi

if [ ! -z "$3" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&companyID=$3"
  else
    QUERY_PARAMS="companyID=$3"
  fi
fi

# Set up curl execution
API="${API_URL}/v4/cla/authorization"
if [ ! -z "$QUERY_PARAMS" ]; then
  API="${API}?${QUERY_PARAMS}"
fi

CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh