#!/bin/bash
# GET /repository-provider/gitlab/sign/{organizationID}/{gitlabRepositoryID}/{mergeRequestID}
# Get GitLab signing URL for merge request (authenticated)
# Usage: ./get_gitlab_sign.sh <organization_id> <gitlab_repository_id> <merge_request_id>
# Example: ./get_gitlab_sign.sh org123 repo456 mr789
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_gitlab_sign.sh <org_id> <repo_id> <mr_id>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify organization_id, gitlab_repository_id, and merge_request_id as parameters"
  echo "Usage: $0 <organization_id> <gitlab_repository_id> <merge_request_id>"
  echo "Example: $0 org123 repo456 mr789"
  exit 1
fi

export organization_id="$1"
export gitlab_repository_id="$2"
export merge_request_id="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/repository-provider/gitlab/sign/${organization_id}/${gitlab_repository_id}/${merge_request_id}"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh