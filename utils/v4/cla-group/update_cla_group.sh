#!/bin/bash
# Update a CLA group (authenticated)
# Usage: ./update_cla_group.sh <cla_group_id> <cla_group_name> <description>
# Example: ./update_cla_group.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e "Updated CLA Group" "Updated description"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./update_cla_group.sh <params>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify cla_group_id, cla_group_name, and description as parameters"
  echo "Usage: $0 <cla_group_id> <cla_group_name> <description>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e \"Updated CLA Group\" \"Updated description\""
  exit 1
fi

export cla_group_id="$1"
export cla_group_name="$2"
export description="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "cla_group_description": "${description}",
  "cla_group_name": "${cla_group_name}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/cla-group/${cla_group_id}"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh