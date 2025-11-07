#!/bin/bash
# GET /company/entityname/{signingEntityName}
# Get company by signing entity name (authenticated)
# Usage: ./get_company_by_entity_name.sh <signing_entity_name>
# Example: ./get_company_by_entity_name.sh "Acme Corporation LLC"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_company_by_entity_name.sh <entity_name>

if [ -z "$1" ]
then
  echo "$0: you need to specify signing_entity_name as a parameter"
  echo "Usage: $0 <signing_entity_name>"
  echo "Example: $0 \"Acme Corporation LLC\""
  exit 1
fi

export signing_entity_name="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# URL encode the entity name
encoded_name=$(printf '%s' "$signing_entity_name" | sed 's/ /%20/g' | sed 's/+/%2B/g' | sed 's/&/%26/g')

# Set up curl execution
API="${API_URL}/v4/company/entityname/${encoded_name}"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh