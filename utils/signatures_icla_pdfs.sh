#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# company_sfid='0016s000006Uq9VAAS'
# TOKEN='...' - Auth0 JWT bearer token
# XACL='...' - X-ACL
# DEBUG=1 XACL="$(cat ./x-acl.secret)" TOKEN="$(cat ./auth0.token.secret)" ./utils/signatures_icla_pdfs.sh 01af041c-fa69-4052-a23c-fb8c1d3bef24

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

if [ -z "$1" ]
then
  echo "$0: you need to specify company_sfid as a 1st parameter"
  exit 1
fi
export cla_group_id="$1"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XPOST -H 'X-ACL: ${XACL}' -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' '${API_URL}/v4/signatures/project/${cla_group_id}/icla/pdfs' | jq -r '.'"
fi
curl -s -XGET -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v4/signatures/project/${cla_group_id}/icla/pdfs" | jq -r '.'
