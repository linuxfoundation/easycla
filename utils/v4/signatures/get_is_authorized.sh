#!/bin/bash
# GET /cla/authorization
# Isauthorized (authenticated)
# Usage: ./get_is_authorized.sh
# API_URL=http://localhost:5001 ./get_is_authorized.sh
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service ./get_is_authorized.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/cla/authorization"
CURL_CMD="curl -s -XGET -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
