#!/bin/bash
# POST /notify-cla-managers
# Notifyclamanagers (authenticated)
# Usage: ./post_notify_cla_managers.sh
# API_URL=http://localhost:5001 ./post_notify_cla_managers.sh
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service ./post_notify_cla_managers.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "example": "value"
}
EOF
)

# Set up curl execution
API="${API_URL}/v4/notify-cla-managers"
CURL_CMD="curl -s -XPOST -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
