#!/bin/bash
# API_URL=https://3f13-147-75-85-27.ngrok-free.app (defaults to localhost:5000)
# https://api.lfcla.dev.platform.linuxfoundation.org/v2/return-url/6168fc0b-705a-4fde-a9dd-d0a4a9c01457?event=signing_complete
# 'event' flag that describes the redirect reason (2nd parameter)
# 7db9a47c-c8fe-4dcb-822d-3f8406b094e3 - has signature_acl = null
# 6168fc0b-705a-4fde-a9dd-d0a4a9c01457 - has signature_acl type LS (list of strings)
# afcf787b-8010-4c43-8bf7-2dbbfa229f2c - has signature_acl type SS (string set)

if [ -z "$1" ]
then
  echo "$0: you need to specify return_url as a 1st parameter, example '6168fc0b-705a-4fde-a9dd-d0a4a9c01457'"
  exit 1
fi
export return_url="$1"

export extra=''
if [ ! -z "$2" ]
then
  export extra="?event=$2"
fi

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

API="${API_URL}/v2/return-url/${return_url}${extra}"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi
