#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org
# Note: To run manually see cla-backend-go/auth/authorizer.go:SecurityAuth() and update accordingly 'LG:'
if [ -z "$1" ]
then
  echo "$0: you need to specify user_id as a 1st parameter, example '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5', '55775b48-69c1-474d-a07a-2a329e7012b4'"
  exit 1
fi
export user_id="$1"

if [ -z "$TOKEN" ]
then
  # source ./auth0_token.secret
  TOKEN="$(cat ./auth0.token.secret)"
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
  export API_URL="http://localhost:5000"
fi

API="${API_URL}/v3/users/${user_id}"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
  curl -s -XGET -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" "${API}" | jq -r '.'
fi
