#!/bin/bash
# Search organizations by company name and/or website name (public endpoint, no auth required)
# Usage: ./search_organization.sh [companyName] [websiteName]
# Example: ./search_organization.sh "Linux Foundation"
# Example: ./search_organization.sh "" "linuxfoundation.org"
# Example: ./search_organization.sh "Linux Foundation" "linuxfoundation.org"
# API_URL=http://localhost:5001 ./search_organization.sh "Linux Foundation"
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./search_organization.sh "Linux Foundation"

if [ -z "$1" ] && [ -z "$2" ]
then
  echo "$0: you need to specify either companyName or websiteName as parameters"
  echo "Usage: $0 [companyName] [websiteName]"
  echo "Examples:"
  echo "  $0 \"Linux Foundation\""
  echo "  $0 \"\" \"linuxfoundation.org\""
  echo "  $0 \"Linux Foundation\" \"linuxfoundation.org\""
  exit 1
fi

export companyName="$1"
export websiteName="$2"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

# Build query parameters
QUERY_PARAMS=""
if [ ! -z "$companyName" ]; then
  QUERY_PARAMS="companyName=$(echo "$companyName" | sed 's/ /%20/g')"
fi
if [ ! -z "$websiteName" ]; then
  if [ ! -z "$QUERY_PARAMS" ]; then
    QUERY_PARAMS="${QUERY_PARAMS}&websiteName=$(echo "$websiteName" | sed 's/ /%20/g')"
  else
    QUERY_PARAMS="websiteName=$(echo "$websiteName" | sed 's/ /%20/g')"
  fi
fi

API="${API_URL}/v3/organization/search"
if [ ! -z "$QUERY_PARAMS" ]; then
  API="${API}?${QUERY_PARAMS}"
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H \"Content-Type: application/json\" \"${API}\""
  curl -s -XGET -H "Content-Type: application/json" "${API}"
else
  curl -s -XGET -H "Content-Type: application/json" "${API}" | jq -r '.'
fi