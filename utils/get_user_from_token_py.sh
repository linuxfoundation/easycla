#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org
# DEBUG='' ./utils/get_user_from_token_py.sh
# For testing locally two options:
# 1) cla/routes.py: 'LG:': return request and cla.auth.fake_authenticate_user(request.headers) - test with fake data
# Or to get a real user data:
# 2a) on local (non remote) computer: ~/get_oauth_token.sh (or ~/get_oauth_token_prod.sh) (will open browser, authenticate to LF, and return token data)
# 2b) edit 'cla/auth.py': uncomment: 'LG: for local environment override', then run server via: clear && AUTH0_USERNAME_CLAIM_CLI='http://lfx.dev/claims/username' yarn serve:ext
# 2c) then TOKEN='value from the get_oauth_token.sh script' DEBUG='' ./utils/get_user_from_token_py.sh

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

API="${API_URL}/v2/user-from-token"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API}" | jq -r '.'
fi
