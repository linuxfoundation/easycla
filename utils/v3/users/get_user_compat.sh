#!/bin/bash
# Get user by ID via public compatibility endpoint (no auth required)
# Usage: ./get_user_compat.sh <user_id>
# Example: ./get_user_compat.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5
# API_URL=http://localhost:5001 ./get_user_compat.sh <user_id>
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./get_user_compat.sh <user_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify user_id as a 1st parameter"
  echo "Usage: $0 <user_id>"
  echo "Example: $0 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5"
  exit 1
fi
export user_id="$1"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

API="${API_URL}/v3/user-compat/${user_id}"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi