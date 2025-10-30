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

API="${API_URL}/v3/users/username/${username}"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
  curl -s -XGET -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" "${API}" | jq -r '.'
fi