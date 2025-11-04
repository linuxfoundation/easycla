#!/bin/bash
# GET /signatures/id/{signatureID}
# Getsignature (authenticated)
# Example: ./get_signature.sh param1
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_signature.sh <signature_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify signature_id as parameter"
  echo "Usage: $0 <signature_id>"
  echo "Example: $0 param1"
  exit 1
fi

export signature_id="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/signatures/id/${signature_id}"
CURL_CMD="curl -s -XGET -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
