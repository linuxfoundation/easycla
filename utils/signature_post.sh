#!/bin/bash
# API_URL=https://[token].ngrok-free.app (defaults to localhost:5000)
# TOKEN='...' - Auth0 JWT bearer token
# BODY='{...}' - signature body

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ -z "$TOKEN" ]
then
  source ./auth0_token.secret
fi

if [ -z "$TOKEN" ]
then
  echo "$0: TOKEN not specified and unable to obtain one"
  exit 1
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XPOST -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' '${API_URL}/v1/signature' -d '${BODY}' | jq -r '.'"
fi
curl -s -XPOST -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v1/signature" -d "${BODY}" | jq -r '.'
