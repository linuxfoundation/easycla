#!/bin/bash
# POST /template/preview
# Generate template preview (authenticated)
# Usage: ./post_template_preview.sh <template_content>
# Example: ./post_template_preview.sh "Template content to preview"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_template_preview.sh <content>

if [ -z "$1" ]
then
  echo "$0: you need to specify template_content as a parameter"
  echo "Usage: $0 <template_content>"
  echo "Example: $0 \"Template content to preview\""
  exit 1
fi

export template_content="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "template_content": "${template_content}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/template/preview"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh