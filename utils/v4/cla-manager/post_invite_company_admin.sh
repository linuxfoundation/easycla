#!/bin/bash
# POST /user/{userID}/invite-company-admin
# Invite Company Admin based on user request to sign CLA (authenticated)
# Usage: ./post_invite_company_admin.sh <user_id> <cla_group_id> <company_id> <contact_admin> <name> <user_email>
# Example: ./post_invite_company_admin.sh user123 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e comp456 true "John Doe" "john@company.com"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_invite_company_admin.sh <user_id> <cla_group_id> <company_id> <contact_admin> <name> <email>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ] || [ -z "$6" ]
then
  echo "$0: you need to specify user_id, cla_group_id, company_id, contact_admin, name, and user_email as parameters"
  echo "Usage: $0 <user_id> <cla_group_id> <company_id> <contact_admin> <name> <user_email>"
  echo "Example: $0 user123 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e comp456 true \"John Doe\" \"john@company.com\""
  exit 1
fi

export user_id="$1"
export cla_group_id="$2"
export company_id="$3"
export contact_admin="$4"
export name="$5"
export user_email="$6"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "claGroupID": "${cla_group_id}",
  "companyID": "${company_id}",
  "contactAdmin": ${contact_admin},
  "name": "${name}",
  "userEmail": "${user_email}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/user/${user_id}/invite-company-admin"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh