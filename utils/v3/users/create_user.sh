#!/bin/bash
# Create a new user (authenticated)
# Usage: ./create_user.sh <userExternalID> <username> <lfEmail> <lfUsername> <githubID> <githubUsername> [admin] [note]
# Example: ./create_user.sh "12345ABC" "testuser123" "testuser123@example.com" "testuser123" "123456" "testuser123gh" false "Test user"
# API_URL=http://localhost:5001 TOKEN="$(cat ./token.secret)" ./create_user.sh "12345ABC" "testuser123" "testuser123@example.com" "testuser123" "123456" "testuser123gh"

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ] || [ -z "$4" ] || [ -z "$5" ] || [ -z "$6" ]
then
  echo "$0: Missing required parameters"
  echo "Usage: $0 <userExternalID> <username> <lfEmail> <lfUsername> <githubID> <githubUsername> [admin] [note]"
  echo "Example: $0 \"12345ABC\" \"testuser123\" \"testuser123@example.com\" \"testuser123\" \"123456\" \"testuser123gh\" false \"Test user\""
  exit 1
fi

export userExternalID="$1"
export username="$2"
export lfEmail="$3"
export lfUsername="$4"
export githubID="$5"
export githubUsername="$6"
export admin="${7:-false}"
export note="${8:-Created via API script}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "userExternalID": "${userExternalID}",
  "username": "${username}",
  "lfEmail": "${lfEmail}",
  "lfUsername": "${lfUsername}",
  "githubID": "${githubID}",
  "githubUsername": "${githubUsername}",
  "admin": ${admin},
  "note": "${note}",
  "emails": ["${lfEmail}"]
}
EOF
)

# Set up curl execution
API="${API_URL}/v3/users"
CURL_CMD="curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh