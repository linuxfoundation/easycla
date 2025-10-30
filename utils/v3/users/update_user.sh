#!/bin/bash
# Update an existing user (authenticated)
# Usage: ./update_user.sh <userID> [note] [email1,email2,...]
# Example: ./update_user.sh "9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5" "Updated note" "email1@example.com,email2@example.com"
# API_URL=http://localhost:5001 TOKEN="$(cat ./token.secret)" ./update_user.sh <userID>

if [ -z "$1" ]
then
  echo "$0: you need to specify userID as a 1st parameter"
  echo "Usage: $0 <userID> [note] [email1,email2,...]"
  echo "Example: $0 \"9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5\" \"Updated note\" \"email1@example.com,email2@example.com\""
  exit 1
fi

export userID="$1"
export note="${2:-Updated via API script}"
export emails_param="${3:-}"

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

# Build emails array
if [ ! -z "$emails_param" ]; then
  # Convert comma-separated emails to JSON array
  emails_json=$(echo "$emails_param" | sed 's/,/","/g' | sed 's/^/"/' | sed 's/$/"/')
  emails_json="[${emails_json}]"
else
  emails_json='[]'
fi

# Build JSON payload
PAYLOAD=$(cat <<EOF
{
  "userID": "${userID}",
  "note": "${note}",
  "emails": ${emails_json}
}
EOF
)

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XPUT -H \"Content-Type: application/json\" \"${API}\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
  curl -s -XPUT -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -d "${PAYLOAD}" "${API}"
else
  curl -s -XPUT -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -d "${PAYLOAD}" "${API}" | jq -r '.'
fi