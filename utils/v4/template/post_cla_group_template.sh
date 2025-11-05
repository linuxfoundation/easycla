#!/bin/bash
# POST /clagroup/{claGroupID}/template
# Create template for CLA group (authenticated)
# Usage: ./post_cla_group_template.sh <cla_group_id> <template_name> <template_content>
# Example: ./post_cla_group_template.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e "My Template" "Template content here"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_cla_group_template.sh <cla_group_id> <name> <content>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify cla_group_id, template_name, and template_content as parameters"
  echo "Usage: $0 <cla_group_id> <template_name> <template_content>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e \"My Template\" \"Template content here\""
  exit 1
fi

export cla_group_id="$1"
export template_name="$2"
export template_content="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "template_name": "${template_name}",
  "template_content": "${template_content}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/clagroup/${cla_group_id}/template"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh