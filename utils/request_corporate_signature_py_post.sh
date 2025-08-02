#!/bin/bash
# API_URL=https://3f13-147-75-85-27.ngrok-free.app (defaults to localhost:5000)
# company_id='862ff296-6508-4f10-9147-2bc2dd7bfe80'
# project_id='88ee12de-122b-4c46-9046-19422054ed8d'
# return_url_type='github'
# return_url='http://localhost'
# TOKEN='...' - Auth0 JWT bearer token
# XACL='...' - X-ACL
# DEBUG=1 ./utils/request_corporate_signature_py_post.sh 862ff296-6508-4f10-9147-2bc2dd7bfe80 88ee12de-122b-4c46-9046-19422054ed8d github 'http://localhost'
# ./utils/request_corporate_signature_py_post.sh 0ca30016-6457-466c-bc41-a09560c1f9bf 88ee12de-122b-4c46-9046-19422054ed8d github 'http://localhost'
# ./utils/request_corporate_signature_py_post.sh 10bde6b1-3061-4972-9c6a-17dd9a175a5c 88ee12de-122b-4c46-9046-19422054ed8d github 'http://localhost'
# Note: this is only for internal usage, it requires 'check_auth' function update in cla-backend/cla/routes.py (see LG:) and can only be tested locally (LG:)
# Note: you can run it in a similar way to utils/get_user_from_token_py.sh

if [ -z "$1" ]
then
  echo "$0: you need to specify company_id as a 2nd parameter"
  exit 1
fi
export company_id="$1"

if [ -z "$2" ]
then
  echo "$0: you need to specify project_id as a 3rd parameter"
  exit 2
fi
export project_id="$2"

if [ -z "$3" ]
then
  echo "$0: you need to specify return_url_type as a 4th parameter: github|gitlab|gerrit"
  exit 3
fi
export return_url_type="$3"

if [ -z "$4" ]
then
  echo "$0: you need to specify return_url as a 5th parameter"
  exit 4
fi
export return_url="$4"

if [ -z "$TOKEN" ]
then
  # source ./auth0_token.secret
  TOKEN="$(cat ./auth0.token.secret)"
fi

if [ -z "$TOKEN" ]
then
  echo "$0: TOKEN not specified and unable to obtain one"
  exit 5
fi

if [ -z "$XACL" ]
then
  XACL="$(cat ./x-acl.secret)"
fi

if [ -z "$XACL" ]
then
  echo "$0: XACL not specified and unable to obtain one"
  exit 6
fi

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XPOST -H 'X-ACL: ${XACL}' -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' '${API_URL}/v1/request-corporate-signature' -d '{\"project_id\":\"${project_id}\",\"company_id\":\"${company_id}\",\"return_url_type\":\"${return_url_type}\",\"return_url\":\"${return_url}\"}' | jq -r '.'"
fi
curl -s -XPOST -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v1/request-corporate-signature" -d "{\"project_id\":\"${project_id}\",\"company_id\":\"${company_id}\",\"return_url_type\":\"${return_url_type}\",\"return_url\":\"${return_url}\"}" | jq -r '.'
