#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org
# REDIRECT=0|1 DEBUG='' ./utils/get_user_from_session_py.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org DEBUG=1 REDIRECT=0 ./utils/get_user_from_session_py.sh 'https://contributor.easycla.lfx.linuxfoundation.org/#/cla/project/68fa91fe-51fe-41ac-a21d-e0a0bf688a53'

if [ -z "$1" ]
then
  echo "$0: you need to specify return URL as a 1st argument, for example: 'https://contributor.easycla.lfx.linuxfoundation.org/#/cla/project/68fa91fe-51fe-41ac-a21d-e0a0bf688a53'"
  exit 1
fi
export redirect_url="$1"
export encoded_redirect_url=$(jq -rn --arg x "$redirect_url" '$x|@uri')

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ -z "${REDIRECT}" ]
then
  export REDIRECT="0"
fi

API="${API_URL}/v2/user-from-session?redirect=${REDIRECT}&redirect_url=${encoded_redirect_url}"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -i -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi
