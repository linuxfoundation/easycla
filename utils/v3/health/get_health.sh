#!/bin/bash
# Get application health status (public endpoint, no auth required)
# Usage: ./get_health.sh
# API_URL=http://localhost:5001 ./get_health.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./get_health.sh

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

API="${API_URL}/v3/ops/health"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi