#!/bin/bash
# Get Swagger JSON specification (public endpoint, no auth required)
# Usage: ./get_swagger_json.sh
# API_URL=http://localhost:5001 ./get_swagger_json.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./get_swagger_json.sh

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

API="${API_URL}/v3/swagger.json"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi