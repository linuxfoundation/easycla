#!/bin/bash
# PUT /cla-group/{claGroupID}/unenroll-projects
# Unenroll projects from a CLA Group (authenticated)
# Usage: ./put_unenroll_projects.sh <cla_group_id> <project_sfid_list>
# Example: ./put_unenroll_projects.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e "a01234567890123456,a09876543210987654"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./put_unenroll_projects.sh <cla_group_id> <project_list>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify cla_group_id and project_sfid_list as parameters"
  echo "Usage: $0 <cla_group_id> <project_sfid_list>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e \"a01234567890123456,a09876543210987654\""
  exit 1
fi

export cla_group_id="$1"
export project_sfid_list="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Convert comma-separated list to JSON array
IFS=',' read -ra PROJECTS <<< "$project_sfid_list"
JSON_ARRAY="["
for i in "${PROJECTS[@]}"; do
  if [ "$JSON_ARRAY" != "[" ]; then
    JSON_ARRAY+=","
  fi
  JSON_ARRAY+="\"$i\""
done
JSON_ARRAY+="]"

# Set up curl execution
API="${API_URL}/v4/cla-group/${cla_group_id}/unenroll-projects"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${JSON_ARRAY}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh