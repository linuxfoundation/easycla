#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# cla_group='01af041c-fa69-4052-a23c-fb8c1d3bef24'
# company_id='1dd8c59d-24cd-4155-9c6f-d4ac15ad5857'
# TOKEN='...' - Auth0 JWT bearer token
# XACL='...' - X-ACL
# DEBUG=1 ./utils/cla_group_corporate_contributors.sh 01af041c-fa69-4052-a23c-fb8c1d3bef24 1dd8c59d-24cd-4155-9c6f-d4ac15ad5857

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
  echo "$0: you need to specify cla_group UUID as a 1st parameter, example: '01af041c-fa69-4052-a23c-fb8c1d3bef24'"
  exit 3
fi
export cla_group="$1"

if [ -z "$2" ]
then
  echo "$0: you need to specify company_id as a 2nd parameter, example: '1dd8c59d-24cd-4155-9c6f-d4ac15ad5857'"
  exit 4
fi
export company_id="$2"

if [ -z "$3" ]
then
  echo "$0: assuming page_size 9999, you can specify page size as a 3rd argument"
  export page_size="9999"
else
  export page_size="$3"
fi

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H 'X-ACL: ${XACL}' -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' '${API_URL}/v4/cla-services/cla-group/${cla_group}/corporate-contributors?companyID=${company_id}&pageSize=${page_size}'"
fi
curl -s -XGET -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v4/cla-group/${cla_group}/corporate-contributors?companyID=${company_id}&pageSize=${page_size}"
