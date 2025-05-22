#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org
# REDIRECT=0|1 DEBUG='' ./utils/get_user_from_session_py.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org DEBUG=1 REDIRECT=0 ./utils/get_user_from_session_py.sh 'https://contributor.easycla.lfx.linuxfoundation.org'
# CODE=xyz STAE=xyz
# ./utils/ngrok.sh then DEBUG='' REDIRECT=0 ./utils/get_user_from_session_py.sh 'https://edc5-147-75-85-27.ngrok-free.app/v2/user-from-session'
# yarn serve:ext:
# DEBUG='' REDIRECT=0 ./utils/get_user_from_session_py.sh 'http://147.75.85.27:5000/v2/user-from-session'
# yarn serve:ext && API_URL='http://147.75.85.27:5000' DEBUG='' REDIRECT=0 ./utils/get_user_from_session_py.sh 'http://147.75.85.27:5000/v2/user-from-session'

export redirect_url="${1}"
export encoded_redirect_url=$(jq -rn --arg x "$redirect_url" '$x|@uri')

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ -z "${REDIRECT}" ]
then
  export REDIRECT="0"
fi

if ( [ -z "${CODE}" ] && [ -z "${STATE}" ] )
then
  export API="${API_URL}/v2/user-from-session?redirect=${REDIRECT}&redirect_url=${encoded_redirect_url}"
else
  export API="${API_URL}/v2/user-from-session?code=${CODE}&state=${STATE}"
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -i -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi
