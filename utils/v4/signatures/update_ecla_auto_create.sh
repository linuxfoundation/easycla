#!/bin/bash
# Update ECLA auto-create flag for company/CLA group (authenticated)
# Usage: ./update_ecla_auto_create.sh <company_id> <cla_group_id> <auto_create_flag>
# Example: ./update_ecla_auto_create.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e e1234567 true
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./update_ecla_auto_create.sh <company_id> <cla_group_id> <flag>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify company_id, cla_group_id, and auto_create_flag as parameters"
  echo "Usage: $0 <company_id> <cla_group_id> <auto_create_flag>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e e1234567 true"
  exit 1
fi

export company_id="$1"
export cla_group_id="$2"
export auto_create_flag="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "auto_create_ecla": ${auto_create_flag}
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/signatures/company/${company_id}/clagroup/${cla_group_id}/ecla-auto-create"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh