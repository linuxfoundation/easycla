#!/bin/bash
# Get signed document for signature (authenticated)
# Usage: ./get_signed_document.sh <signature_id>
# Example: ./get_signed_document.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_signed_document.sh <signature_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify signature_id as a parameter"
  echo "Usage: $0 <signature_id>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e"
  exit 1
fi

export signature_id="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/signatures/${signature_id}/signed-document"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=false
. ./utils/shared/handle_curl_execution.sh