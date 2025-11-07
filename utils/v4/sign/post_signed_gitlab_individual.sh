#!/bin/bash
# POST /signed/gitlab/individual/{user_id}/{organization_id}/{gitlab_repository_id}/{merge_request_id}
# Submit signed GitLab individual CLA (authenticated)
# Usage: ./post_signed_gitlab_individual.sh <user_id> <org_id> <repo_id> <mr_id> <signature_data>
# Example: ./post_signed_gitlab_individual.sh user123 org456 repo789 mr001 "signature_data_here"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_signed_gitlab_individual.sh <user> <org> <repo> <mr> <data>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ]
then
  echo "$0: you need to specify user_id, organization_id, gitlab_repository_id, merge_request_id, and signature_data as parameters"
  echo "Usage: $0 <user_id> <organization_id> <gitlab_repository_id> <merge_request_id> <signature_data>"
  echo "Example: $0 user123 org456 repo789 mr001 \"signature_data_here\""
  exit 1
fi

export user_id="$1"
export organization_id="$2"
export gitlab_repository_id="$3"
export merge_request_id="$4"
export signature_data="$5"

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
API="${API_URL}/v4/signed/gitlab/individual/${user_id}/${organization_id}/${gitlab_repository_id}/${merge_request_id}"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh