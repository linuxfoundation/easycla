#!/bin/bash
# Get user by ID (authenticated)
# Usage: ./get_user.sh <user_id>
# Example: ./get_user.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5
# API_URL=http://localhost:5001 TOKEN="$(cat ./token.secret)" ./get_user.sh <user_id>
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org TOKEN="$(cat ./token.secret)" ./get_user.sh <user_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify user_id as a 1st parameter"
  echo "Usage: $0 <user_id>"
  echo "Example: $0 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5"
  exit 1
fi
export user_id="$1"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ./utils/shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v3/users/${user_id}"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh