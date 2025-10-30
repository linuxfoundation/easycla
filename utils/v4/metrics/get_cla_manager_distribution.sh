#!/bin/bash
# Get CLA manager distribution for EasyCLA (authenticated)
# Usage: ./get_cla_manager_distribution.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_cla_manager_distribution.sh
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_cla_manager_distribution.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/metrics/cla-manager-distribution"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh