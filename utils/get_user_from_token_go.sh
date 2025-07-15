#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org
# DEBUG='' ./utils/get_user_from_token_go.sh
# Note: To run manually see cla-backend-go/auth/authorizer.go:SecurityAuth() and update accordingly 'LG:'
# Or generate a real token using ... and the edit 'cla-backend-go/cmd/server.go' - look for "LG: to test with manual tokens"
# Or to get a real user data:
# on local (non remote) computer: ~/get_oauth_token.sh (or ~/get_oauth_token_prod.sh) (will open browser, authenticate to LF, and return token data)
# edit 'cla-backend-go/cmd/server.go' - look for "LG: to test with manual tokens", then 'cla-backend-go/auth/authorizer.go': LG: to allow local testing", then run ./bin/cla
# then TOKEN='value from the get_oauth_token.sh script' DEBUG='' ./utils/get_user_from_token_go.sh
# DEBUG=1 API_URL='https://api-gw.dev.platform.linuxfoundation.org/cla-service' TOKEN='...' ./utils/get_user_from_token_go.sh
# DEBUG=1 API_URL='http://localhost:5001/cla-service' TOKEN='...' ./utils/get_user_from_token_go.sh

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

API="${API_URL}/v4/user-from-token"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API}" | jq -r '.'
fi
