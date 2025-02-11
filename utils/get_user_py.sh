#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org
if [ -z "$1" ]
then
  echo "$0: you need to specify user_id as a 1st parameter, example '9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5', '55775b48-69c1-474d-a07a-2a329e7012b4'"
  exit 1
fi
export user_id="$1"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

API="${API_URL}/v2/user/${user_id}"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi
