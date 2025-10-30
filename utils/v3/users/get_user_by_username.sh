#!/bin/bash
# Get user by username (authenticated)
# Usage: ./get_user_by_username.sh <username>
# Example: ./get_user_by_username.sh lukaszgryglicki
# API_URL=http://localhost:5001 TOKEN="$(cat ./token.secret)" ./get_user_by_username.sh <username>
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org TOKEN="$(cat ./token.secret)" ./get_user_by_username.sh <username>

if [ -z "$1" ]
then
  echo "$0: you need to specify username as a 1st parameter"
  echo "Usage: $0 <username>"
  echo "Example: $0 lukaszgryglicki"
  exit 1
fi
export username="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v3/users/username/${username}"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh