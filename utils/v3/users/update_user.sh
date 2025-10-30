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

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ./utils/shared/handle_api_url.sh

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

# Set up curl execution
API="${API_URL}/v3/users"
CURL_CMD="curl -s -XPUT -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh