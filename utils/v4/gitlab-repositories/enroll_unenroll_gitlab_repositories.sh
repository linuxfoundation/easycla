#!/bin/bash
# Enroll/Unenroll GitLab repositories for CLA enforcement (authenticated)
# Usage: ./enroll_unenroll_gitlab_repositories.sh <project_sfid> <cla_group_id> <action> <repo_id1> [repo_id2 ...]
# Example: ./enroll_unenroll_gitlab_repositories.sh a09P000000DsNH2IAN d9428888-122b-4b20-8c4a-0c9a1a6f9b8e enroll 12345 67890
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./enroll_unenroll_gitlab_repositories.sh <params>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ]
then
  echo "$0: you need to specify project_sfid, cla_group_id, action, and at least one repository_id as parameters"
  echo "Usage: $0 <project_sfid> <cla_group_id> <action> <repo_id1> [repo_id2 ...]"
  echo "Actions: enroll, unenroll"
  echo "Example: $0 a09P000000DsNH2IAN d9428888-122b-4b20-8c4a-0c9a1a6f9b8e enroll 12345 67890"
  exit 1
fi

export project_sfid="$1"
export cla_group_id="$2"
export action="$3"
shift 3  # Remove first 3 parameters, leaving repo IDs

# Build array of repository IDs
repo_ids=""
for repo_id in "$@"; do
  if [ -z "$repo_ids" ]; then
    repo_ids="$repo_id"
  else
    repo_ids="$repo_ids, $repo_id"
  fi
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "cla_group_id": "${cla_group_id}",
  "${action}": [${repo_ids}]
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/project/${project_sfid}/gitlab/repositories"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh