#!/bin/bash
# POST /cla-group/{claGroupID}/project/{projectSFID}/gerrits
# Add Gerrit repository to a project in CLA group (authenticated)
# Usage: ./add_project_gerrit.sh <cla_group_id> <project_sfid> <gerrit_name> <gerrit_url> <group_id_icla> <group_id_ccla>
# Example: ./add_project_gerrit.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a09P000000DsNH2IAN "My Gerrit" "https://gerrit.example.com" "group1" "group2"
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./add_project_gerrit.sh <params>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ] || [ -z "$6" ]
then
  echo "$0: you need to specify cla_group_id, project_sfid, gerrit_name, gerrit_url, group_id_icla, and group_id_ccla as parameters"
  echo "Usage: $0 <cla_group_id> <project_sfid> <gerrit_name> <gerrit_url> <group_id_icla> <group_id_ccla>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e a09P000000DsNH2IAN \"My Gerrit\" \"https://gerrit.example.com\" \"group1\" \"group2\""
  exit 1
fi

export cla_group_id="$1"
export project_sfid="$2"
export gerrit_name="$3"
export gerrit_url="$4"
export group_id_icla="$5"
export group_id_ccla="$6"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "gerrit_name": "${gerrit_name}",
  "gerrit_url": "${gerrit_url}",
  "group_id_icla": "${group_id_icla}",
  "group_id_ccla": "${group_id_ccla}"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/cla-group/${cla_group_id}/project/${project_sfid}/gerrits"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh