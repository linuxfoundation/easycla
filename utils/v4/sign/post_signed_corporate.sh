#!/bin/bash
# POST /signed/corporate/{project_id}/{company_id}
# Submit signed corporate CLA (authenticated)
# Usage: ./post_signed_corporate.sh <project_id> <company_id> <signature_data>
# Example: ./post_signed_corporate.sh proj123 comp456 "signature_data_here"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_signed_corporate.sh <proj> <comp> <data>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify project_id, company_id, and signature_data as parameters"
  echo "Usage: $0 <project_id> <company_id> <signature_data>"
  echo "Example: $0 proj123 comp456 \"signature_data_here\""
  exit 1
fi

export project_id="$1"
export company_id="$2"
export signature_data="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "signature_data": "${signature_data}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/signed/corporate/${project_id}/${company_id}"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh