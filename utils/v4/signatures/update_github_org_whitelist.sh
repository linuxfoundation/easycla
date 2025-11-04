#!/bin/bash
# Update GitHub organization whitelist for signature (authenticated)
# Usage: ./update_github_org_whitelist.sh <signature_id> <organization_id>
# Example: ./update_github_org_whitelist.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e 35275118
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./update_github_org_whitelist.sh <signature_id> <org_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify signature_id and organization_id as parameters"
  echo "Usage: $0 <signature_id> <organization_id>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e 35275118"
  exit 1
fi

export signature_id="$1"
export organization_id="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "organization_id": "${organization_id}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/signatures/${signature_id}/gh-org-whitelist"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh