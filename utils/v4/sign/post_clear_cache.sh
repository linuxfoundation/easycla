#!/bin/bash
# POST /clear-cache
# Clear cache (authenticated)
# Usage: ./post_clear_cache.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./post_clear_cache.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/clear-cache"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '{}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh