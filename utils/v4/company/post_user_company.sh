#!/bin/bash
# POST /user/{userID}/company
# Creates a new salesforce company (authenticated)
# Usage: ./post_user_company.sh <user_id> <company_name> <company_website> <user_email> <signing_entity_name>
# Example: ./post_user_company.sh user123 "Acme Corp" "https://acme.com" "founder@acme.com" "Acme Corporation"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_user_company.sh <user_id> <name> <website> <email> <entity>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ]
then
  echo "$0: you need to specify user_id, company_name, company_website, user_email, and signing_entity_name as parameters"
  echo "Usage: $0 <user_id> <company_name> <company_website> <user_email> <signing_entity_name>"
  echo "Example: $0 user123 \"Acme Corp\" \"https://acme.com\" \"founder@acme.com\" \"Acme Corporation\""
  exit 1
fi

export user_id="$1"
export company_name="$2"
export company_website="$3"
export user_email="$4"
export signing_entity_name="$5"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "companyName": "${company_name}",
  "companyWebsite": "${company_website}",
  "userEmail": "${user_email}",
  "signingEntityName": "${signing_entity_name}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/user/${user_id}/company"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh