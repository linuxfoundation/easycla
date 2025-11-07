#!/bin/bash
# POST /cla-group/validate
# Validate CLA group configuration (authenticated)
# Usage: ./post_validate_cla_group.sh <validation_data>
# Example: ./post_validate_cla_group.sh '{"cla_group_name": "test", "foundation_sfid": "a01234567890123456"}'
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_validate_cla_group.sh <validation_data>

if [ -z "$1" ]
then
  echo "$0: you need to specify validation_data as a parameter"
  echo "Usage: $0 <validation_data>"
  echo "Example: $0 '{\"cla_group_name\": \"test\", \"foundation_sfid\": \"a01234567890123456\"}'"
  exit 1
fi

export validation_data="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/cla-group/validate"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${validation_data}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh