#!/bin/bash
# Get project metrics of the top projects (authenticated)
# Usage: ./get_top_projects.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_top_projects.sh
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_top_projects.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/metrics/top-projects"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh