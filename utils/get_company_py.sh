#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org
if [ -z "$1" ]
then
  echo "$0: you need to specify company_id as a 1st parameter, example '0ca30016-6457-466c-bc41-a09560c1f9bf', '10bde6b1-3061-4972-9c6a-17dd9a175a5c'"
  exit 1
fi
export company_id="$1"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

API="${API_URL}/v2/company/${company_id}"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi
