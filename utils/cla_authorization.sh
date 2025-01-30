#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# cla_group='b71c469a-55e7-492c-9235-fd30b31da2aa' (ONAP)
# lfid='lflgryglicki'
# TOKEN='...' - Auth0 JWT bearer token
# XACL='...' - X-ACL
# API_URL='https://api-gw.platform.linuxfoundation.org/cla-service/v4/cla-service/v4/'
# DEBUG=1 ./utils/cla_group_corporate_contributors.sh b71c469a-55e7-492c-9235-fd30b31da2aa andreasgeissler

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
  echo "$0: you need to specify cla_group UUID as a 1st parameter, example: 'b71c469a-55e7-492c-9235-fd30b31da2aa', '01af041c-fa69-4052-a23c-fb8c1d3bef24'"
  exit 3
fi
export cla_group="$1"

if [ -z "$2" ]
then
  echo "$0: you need to specify lfid as a 2nd parameter, example: 'andreasgeissler'"
  exit 4
fi
export lfid="$2"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H 'X-ACL: ${XACL}' -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' '${API_URL}/v4/cla-services/cla/authorization?lfid=${lfid}&claGroupId=${cla_group}'"
fi
curl -s -XGET -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v4/cla/authorization?lfid=${lfid}&claGroupId=${cla_group}"
