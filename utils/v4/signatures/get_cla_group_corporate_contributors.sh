#!/bin/bash
# Get corporate contributors for a CLA group (authenticated)
# Usage: ./get_cla_group_corporate_contributors.sh <cla_group_id> [company_id] [pageSize]
# Example: ./get_cla_group_corporate_contributors.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a01234567890123456 100
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_cla_group_corporate_contributors.sh <cla_group_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify cla_group_id as a parameter"
  echo "Usage: $0 <cla_group_id> [company_id] [pageSize]"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a01234567890123456 100"
  exit 1
fi

export cla_group_id="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build query parameters
QUERY_PARAMS=""
if [ ! -z "$2" ]; then
  QUERY_PARAMS="companyID=$2"
fi

if [ ! -z "$3" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&pageSize=$3"
  else
    QUERY_PARAMS="pageSize=$3"
  fi
fi

# Set up curl execution
API="${API_URL}/v4/cla-group/${cla_group_id}/corporate-contributors"
if [ ! -z "$QUERY_PARAMS" ]; then
  API="${API}?${QUERY_PARAMS}"
fi

CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh