#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# company_sfid='0016s000006Uq9VAAS'
# project_sfid='a092h000004wx1DAAQ'
# return_url_type='github'
# return_url='http://localhost'
# TOKEN='...' - Auth0 JWT bearer token
# XACL='...' - X-ACL
# DEBUG=1 XACL="$(cat ./x-acl.secret)" TOKEN="$(cat ./auth0.token.secret)" ./utils/request_corporate_signature_go_post.sh 0016s000006Uq9VAAS a092h000004wx1DAAQ github 'http://localhost'

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

if [ -z "$1" ]
then
  echo "$0: you need to specify company_sfid as a 1st parameter"
  exit 1
fi
export company_sfid="$1"

if [ -z "$2" ]
then
  echo "$0: you need to specify project_sfid as a 2nd parameter"
  exit 2
fi
export project_sfid="$2"

if [ -z "$3" ]
then
  echo "$0: you need to specify return_url_type as a 3rd parameter: github|gitlab|gerrit"
  exit 3
fi
export return_url_type="$3"

if [ -z "$4" ]
then
  echo "$0: you need to specify return_urlas a 4th parameter"
  exit 4
fi
export return_url="$4"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XPOST -H 'X-ACL: ${XACL}' -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' '${API_URL}/v4/request-corporate-signature' -d '{\"project_sfid\":\"${project_sfid}\",\"company_sfid\":\"${company_sfid}\",\"return_url_type\":\"${return_url_type}\",\"return_url\":\"${return_url}\"}' | jq -r '.'"
fi
curl -s -XPOST -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v4/request-corporate-signature" -d "{\"project_sfid\":\"${project_sfid}\",\"company_sfid\":\"${company_sfid}\",\"return_url_type\":\"${return_url_type}\",\"return_url\":\"${return_url}\"}" | jq -r '.'
