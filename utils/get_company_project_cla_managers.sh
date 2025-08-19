#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# API_URL=https://api-gw.platform.linuxfoundation.org/cla-service
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service
# DEBUG='' ./utils/get_company_project_cla_managers.sh f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc a09P000000DsCE5IAN
# Note: To run manually see cla-backend-go/auth/authorizer.go:SecurityAuth() and update accordingly 'LG:'
# STAGE=prod ./utils/scan.sh projects-cla-groups project_name 'Cloud Native Computing Foundation (CNCF)'
# STAGE=prod ./utils/scan.sh companies company_name 'Red Hat, Inc.'
# ./get_oauth_token_prod.sh
# DEBUG='' API_URL=https://api-gw.platform.linuxfoundation.org/cla-service ./utils/get_company_project_cla_managers.sh '86fda258-9866-45e3-b4fe-50502ac0957c' 'a0941000002wBz4AAE'

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

if [ -z "${1}" ]
then
  echo "$0: you need to provide company UUID as a 1st argument, for example: 'f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc'"
  exit 3
fi
export company_uuid="${1}"

if [ -z "${2}" ]
then
  echo "$0: you need to provide project SFID as a 2nd argument, for example: 'a09P000000DsCE5IAN'"
  exit 4
fi
export project_sfid="${2}"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

API="${API_URL}/v4/company/${company_uuid}/project/${project_sfid}/cla-managers"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\" -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API}" | jq -r '.'
fi
