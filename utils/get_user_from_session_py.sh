#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org
# REDIRECT=0|1 DEBUG='' ./utils/get_user_from_session_py.sh

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ -z "${REDIRECT}" ]
then
  export REDIRECT="0"
fi

API="${API_URL}/v2/user-from-session?redirect=${REDIRECT}"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi
