#!/bin/bash
# GET /template/{claGroupID}/preview
# Get template preview for CLA group (authenticated)
# Usage: ./get_template_preview.sh <cla_group_id>
# Example: ./get_template_preview.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_template_preview.sh <cla_group_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify cla_group_id as a parameter"
  echo "Usage: $0 <cla_group_id>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e"
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
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh