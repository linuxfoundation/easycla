#!/bin/bash
# GET /project/external/{externalID}
# Getprojectsbyexternalid (authenticated)
# Example: ./get_projects_by_external_id.sh param1
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_projects_by_external_id.sh <external_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify external_id as parameter"
  echo "Usage: $0 <external_id>"
  echo "Example: $0 param1"
  exit 1
fi

export external_id="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/project/external/${external_id}"
CURL_CMD="curl -s -XGET -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
