#!/bin/bash
# Get API Documentation (public endpoint, no auth required)
# Usage: ./get_api_docs.sh
# API_URL=http://localhost:5001 ./get_api_docs.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./get_api_docs.sh

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

API="${API_URL}/v3/api-docs"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}"
fi