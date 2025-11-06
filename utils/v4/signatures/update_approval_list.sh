#!/bin/bash
# Update approval list for project/company/CLA group (authenticated)
# Usage: ./update_approval_list.sh <project_sfid> <company_id> <cla_group_id> <operation> <type> <value>
# Example: ./update_approval_list.sh a01234567890123456 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e e1234567 AddEmailApprovalList test@example.com
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./update_approval_list.sh <params>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ] || [ -z "$6" ]
then
  echo "$0: you need to specify project_sfid, company_id, cla_group_id, operation, type, and value as parameters"
  echo "Usage: $0 <project_sfid> <company_id> <cla_group_id> <operation> <type> <value>"
  echo "Operations: Add, Remove"
  echo "Types: EmailApprovalList, GithubOrgApprovalList, GithubUsernameApprovalList, GitlabUsernameApprovalList, GitlabOrgApprovalList, DomainApprovalList"
  echo "Example: $0 a01234567890123456 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e e1234567 Add EmailApprovalList test@example.com"
  exit 1
fi

export project_sfid="$1"
export company_id="$2"
export cla_group_id="$3"
export operation="$4"
export approval_type="$5"
export value="$6"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build the operation key
operation_key="${operation}${approval_type}"

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "${operation_key}": ["${value}"]
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/signatures/project/${project_sfid}/company/${company_id}/clagroup/${cla_group_id}/approval-list"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh