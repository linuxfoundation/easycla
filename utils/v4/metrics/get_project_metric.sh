#!/bin/bash
# Get metrics of a specific project (authenticated)
# Usage: ./get_project_metric.sh <project_id> [id_type]
# Example: ./get_project_metric.sh a09P000000DsNH2IAN salesforce
# Example: ./get_project_metric.sh project-uuid
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_project_metric.sh <project_id> [id_type]

if [ -z "$1" ]
then
  echo "$0: you need to specify project_id as a 1st parameter"
  echo "Usage: $0 <project_id> [id_type]"
  echo "Example: $0 a09P000000DsNH2IAN salesforce"
  echo "Example: $0 project-uuid"
  exit 1
fi
export project_id="$1"
export id_type="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build query parameters
QUERY_PARAMS=""
if [ ! -z "$id_type" ]; then
  QUERY_PARAMS="idType=${id_type}"
fi

# Set up curl execution
API="${API_URL}/v4/metrics/project/${project_id}"
if [ ! -z "$QUERY_PARAMS" ]; then
  API="${API}?${QUERY_PARAMS}"
fi

CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh