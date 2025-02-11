#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# company_id='1dd8c59d-24cd-4155-9c6f-d4ac15ad5857'
# project_sfid='a09P000000DsCE5IAN'
# TOKEN='...' - Auth0 JWT bearer token
# XACL='...' - X-ACL
# DEBUG=1 ./utils/company_project_contributors.sh 1dd8c59d-24cd-4155-9c6f-d4ac15ad5857 a09P000000DsCE5IAN

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
  echo "$0: you need to specify company_id as a 1st parameter, example: '1dd8c59d-24cd-4155-9c6f-d4ac15ad5857'"
  exit 3
fi
export company_id="$1"

if [ -z "$2" ]
then
  echo "$0: you need to specify project_sfid as a 2nd parameter, example: 'a09P000000DsCE5IAN'"
  exit 4
fi
export project_sfid="$2"


if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H 'X-ACL: ${XACL}' -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' '${API_URL}/v4/company/${company_id}/project/${project_sfid}/contributors'"
fi
curl -s -XGET -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v4/company/${company_id}/project/${project_sfid}/contributors"
