#!/bin/bash
# Invalidate user ICLA for CLA group (authenticated)
# Usage: ./invalidate_user_icla.sh <cla_group_id> <user_id>
# Example: ./invalidate_user_icla.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e e1234567-abcd-4567-8901-234567890abc
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./invalidate_user_icla.sh <cla_group_id> <user_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify cla_group_id and user_id as parameters"
  echo "Usage: $0 <cla_group_id> <user_id>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e e1234567-abcd-4567-8901-234567890abc"
  exit 1
fi

export cla_group_id="$1"
export user_id="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/cla-group/${cla_group_id}/user/${user_id}/icla"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh