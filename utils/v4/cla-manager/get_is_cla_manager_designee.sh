#!/bin/bash
# GET /company/{companySFID}/user/{userLFID}/claGroupID/{claGroupID}/is-cla-manager-designee
# Isclamanagerdesignee (authenticated)
# Example: ./get_is_cla_manager_designee.sh param1 param2 param3
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_is_cla_manager_designee.sh <company_sfid> <user_lfid> <cla_group_id>

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify company_sfid, user_lfid, cla_group_id as parameters"
  echo "Usage: $0 <company_sfid> <user_lfid> <cla_group_id>"
  echo "Example: $0 param1 param2 param3"
  exit 1
fi

export company_sfid="$1"
export user_lfid="$2"
export cla_group_id="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/company/${company_sfid}/user/${user_lfid}/claGroupID/${cla_group_id}/is-cla-manager-designee"
CURL_CMD="curl -s -XGET -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
