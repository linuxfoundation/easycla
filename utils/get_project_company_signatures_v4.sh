#!/bin/bash
# API_URL=https://api-gw.platform.linuxfoundation.org/cla-service
# DEBUG='' API_URL=https://api-gw.platform.linuxfoundation.org/cla-service companyId='5e8f3a01-b117-4c10-ac26-9dec430bf9c5' projectId='a092M00001JwBlEQAV' ./utils/get_project_company_signatures_v4.sh
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

if [ -z "${companyId}" ]
then
  echo "$0: you need to specify companyId='...'"
  exit 3
fi

if [ -z "${projectId}" ]
then
  echo "$0: you need to specify projectId='...'"
  exit 4
fi

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

export API_PATH="v4/signatures/project/${projectId}/company/${companyId}"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H 'X-ACL: ${XACL}' -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' '${API_URL}/${API_PATH}' | jq -r '.'"
  curl -s -XGET -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/${API_PATH}"
else
  curl -s -XGET -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/${API_PATH}" | jq -r '.'
fi
