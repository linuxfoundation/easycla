#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# API_URL=https://api.dev.lfcla.com
# https://api.dev.lfcla.com/v2/gerrit/01af041c-fa69-4052-a23c-fb8c1d3bef24/corporate/agreementUrl.html

if [ -z "$1" ]
then
  echo "$0: you need to specify agreement type: corporate|individual"
  exit 1
fi
export agreement_type="$1"

if [ -z "$2" ]
then
  echo "$0: you need to specify gerrit instance id as a 1st parameter, example '01af041c-fa69-4052-a23c-fb8c1d3bef24'"
  exit 1
fi
export gerrit_id="$2"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

API="${API_URL}/v2/gerrit/${gerrit_id}/${agreement_type}/agreementUrl.html"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi
