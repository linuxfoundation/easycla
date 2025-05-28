#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org
# DEBUG='' ./utils/get_user_from_session_py.sh
# Flow with custom GitHub app: see 'LG:' in cla/controllers/repository_service.py, then:
# Start server via: CLA_API_BASE_CLI='http://147.75.85.27:5000' GH_OAUTH_CLIENT_ID_CLI="$(cat ../lg-github-oauth-app.client-id.secret)" GH_OAUTH_SECRET_CLI="$(cat ../lg-github-oauth-app.client-secret.secret)" yarn serve:ext
# In the browser: open page: http://147.75.85.27:5000/v2/user-from-session

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

export API="${API_URL}/v2/user-from-session"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -i -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi
