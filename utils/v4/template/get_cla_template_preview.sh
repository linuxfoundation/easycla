#!/bin/bash
# GET /template/{claGroupID}/preview
# Getclatemplatepreview (authenticated)
# Example: ./get_cla_template_preview.sh param1
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_cla_template_preview.sh <cla_group_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify cla_group_id as parameter"
  echo "Usage: $0 <cla_group_id>"
  echo "Example: $0 param1"
  exit 1
fi

export cla_group_id="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/template/${cla_group_id}/preview"
CURL_CMD="curl -s -XGET -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
