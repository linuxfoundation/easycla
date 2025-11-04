#!/bin/bash
# POST /company/{companySFID}/contributorAssociation
# Associates a contributor with a company (authenticated)
# Usage: ./post_contributor_association.sh <company_sfid> <user_email>
# Example: ./post_contributor_association.sh a01234567890123456 "contributor@example.com"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_contributor_association.sh <company_sfid> <user_email>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify company_sfid and user_email as parameters"
  echo "Usage: $0 <company_sfid> <user_email>"
  echo "Example: $0 a01234567890123456 \"contributor@example.com\""
  exit 1
fi

export company_sfid="$1"
export user_email="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "userEmail": "${user_email}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/company/${company_sfid}/contributorAssociation"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh