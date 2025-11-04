#!/bin/bash
# POST /signed/individual/{installation_id}/{github_repository_id}/{change_request_id}
# Submit signed individual CLA (authenticated)
# Usage: ./post_signed_individual.sh <installation_id> <github_repository_id> <change_request_id> <signature_data>
# Example: ./post_signed_individual.sh inst123 repo456 pr789 "signature_data_here"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_signed_individual.sh <inst> <repo> <pr> <data>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ]
then
  echo "$0: you need to specify installation_id, github_repository_id, change_request_id, and signature_data as parameters"
  echo "Usage: $0 <installation_id> <github_repository_id> <change_request_id> <signature_data>"
  echo "Example: $0 inst123 repo456 pr789 \"signature_data_here\""
  exit 1
fi

export installation_id="$1"
export github_repository_id="$2"
export change_request_id="$3"
export signature_data="$4"

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
API="${API_URL}/v4/signed/individual/${installation_id}/${github_repository_id}/${change_request_id}"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh