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

if [ -z "$TOKEN" ]
then
  TOKEN="$(cat ./token.secret)"
fi

if [ -z "$TOKEN" ]
then
  echo "$0: TOKEN not specified and unable to obtain one"
  exit 1
fi

if [ -z "$XACL" ]
then
  XACL="$(cat ./x-acl.secret)"
fi

if [ -z "$XACL" ]
then
  echo "$0: XACL not specified and unable to obtain one"
  exit 2
fi

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

API="${API_URL}/v3/users"

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

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XPOST -H \"Content-Type: application/json\" \"${API}\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
  curl -s -XPOST -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -d "${PAYLOAD}" "${API}"
else
  curl -s -XPOST -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -d "${PAYLOAD}" "${API}" | jq -r '.'
fi