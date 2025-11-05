#!/bin/bash
# POST /signatures/{signatureID}/gh-org-whitelist
# Add to GitHub organization whitelist for signature (authenticated)
# Usage: ./post_github_org_whitelist.sh <signature_id> <organization_list>
# Example: ./post_github_org_whitelist.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e "org1,org2,org3"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_github_org_whitelist.sh <signature_id> <org_list>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify signature_id and organization_list as parameters"
  echo "Usage: $0 <signature_id> <organization_list>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e \"org1,org2,org3\""
  exit 1
fi

export signature_id="$1"
export organization_list="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Convert comma-separated list to JSON array
IFS=',' read -ra ORGS <<< "$organization_list"
JSON_ARRAY="["
for i in "${ORGS[@]}"; do
  if [ "$JSON_ARRAY" != "[" ]; then
    JSON_ARRAY+=","
  fi
  JSON_ARRAY+="\"$i\""
done
JSON_ARRAY+="]"

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "list": ${JSON_ARRAY}
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/signatures/${signature_id}/gh-org-whitelist"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh