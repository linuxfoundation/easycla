#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org
# API_URL=https://api.easycla.lfx.linuxfoundation.org
# https://api.easycla.lfx.linuxfoundation.org/v2/project/d8cead54-92b7-48c5-a2c8-b1e295e8f7f1
if [ -z "$1" ]
then
  echo "$0: you need to specify project_id as a 1st parameter, example 'd8cead54-92b7-48c5-a2c8-b1e295e8f7f1'"
  exit 1
fi
export project_id="$1"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

API="${API_URL}/v2/project/${project_id}"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi
