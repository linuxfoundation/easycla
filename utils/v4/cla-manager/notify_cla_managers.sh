#!/bin/bash
# Send notification to CLA managers (authenticated)
# Usage: ./notify_cla_managers.sh <cla_group_id> <company_name> <signing_entity_name> <user_id> <manager_email> <manager_name>
# Example: ./notify_cla_managers.sh e1234567-abcd-4567-8901-234567890abc "Acme Corp" "Acme LLC" d9428888-122b-4b20-8c4a-0c9a1a6f9b8e john.doe@example.com "John Doe"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./notify_cla_managers.sh <params>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ] || [ -z "$6" ]
then
  echo "$0: you need to specify cla_group_id, company_name, signing_entity_name, user_id, manager_email, and manager_name as parameters"
  echo "Usage: $0 <cla_group_id> <company_name> <signing_entity_name> <user_id> <manager_email> <manager_name>"
  echo "Example: $0 e1234567-abcd-4567-8901-234567890abc \"Acme Corp\" \"Acme LLC\" d9428888-122b-4b20-8c4a-0c9a1a6f9b8e john.doe@example.com \"John Doe\""
  exit 1
fi

export cla_group_id="$1"
export company_name="$2"
export signing_entity_name="$3"
export user_id="$4"
export manager_email="$5"
export manager_name="$6"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "claGroupID": "${cla_group_id}",
  "companyName": "${company_name}",
  "signingEntityName": "${signing_entity_name}",
  "userID": "${user_id}",
  "list": [
    {
      "email": "${manager_email}",
      "name": "${manager_name}"
    }
  ]
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/notify-cla-managers"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=false
. ./utils/shared/handle_curl_execution.sh