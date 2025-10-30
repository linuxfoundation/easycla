#!/bin/bash
# Search users with optional parameters
# Usage: ./search_users.sh [searchTerm] [fullMatch] [pageSize]
# Example: ./search_users.sh lukasz true 50
# API_URL=http://localhost:5001 TOKEN="$(cat ./token.secret)" ./search_users.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org TOKEN="$(cat ./token.secret)" ./search_users.sh

if [ -z "$TOKEN" ]
then
  TOKEN="$(cat ./token.secret)"
fi

if [ -z "$TOKEN" ]
then
  echo "$0: TOKEN not specified and unable to obtain one"
  exit 1
fi

if [ -z "$XACL" ]
then
  XACL="$(cat ./x-acl.secret)"
fi

if [ -z "$XACL" ]
then
  echo "$0: XACL not specified and unable to obtain one"
  exit 2
fi

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

# Build query parameters
QUERY_PARAMS=""
if [ ! -z "$1" ]; then
  QUERY_PARAMS="searchTerm=$1"
fi
if [ ! -z "$2" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&fullMatch=$2"
  else
    QUERY_PARAMS="fullMatch=$2"
  fi
fi
if [ ! -z "$3" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&pageSize=$3"
  else
    QUERY_PARAMS="pageSize=$3"
  fi
fi

API="${API_URL}/v3/users/search"
if [ ! -z "$QUERY_PARAMS" ]; then
  API="${API}?${QUERY_PARAMS}"
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
  curl -s -XGET -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" "${API}" | jq -r '.'
fi