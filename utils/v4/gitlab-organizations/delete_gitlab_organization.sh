#!/bin/bash
# Delete GitLab organization from project (authenticated)
# Usage: ./delete_gitlab_organization.sh <project_sfid> <organization_full_path>
# Example: ./delete_gitlab_organization.sh a09P000000DsNH2IAN "myorg/myproject"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./delete_gitlab_organization.sh <project_sfid> <org_path>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify project_sfid and organization_full_path as parameters"
  echo "Usage: $0 <project_sfid> <organization_full_path>"
  echo "Example: $0 a09P000000DsNH2IAN \"myorg/myproject\""
  exit 1
fi

export project_sfid="$1"
export organization_full_path="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution (URL encode the organization path)
encoded_path=$(printf '%s' "$organization_full_path" | sed 's/ /%20/g')
API="${API_URL}/v4/project/${project_sfid}/gitlab/organization?organization_full_path=${encoded_path}"
CURL_CMD="curl -s -XDELETE -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=false
. ./utils/shared/handle_curl_execution.sh