#!/bin/bash
# Shared handler for curl execution with DEBUG support
# Expects variables: API (curl URL), CURL_CMD (curl command), USE_JQ (true/false)
# Usage: 
#   API="https://example.com/api"
#   CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\""
#   USE_JQ=true
#   . ./utils/shared/handle_curl_execution.sh

if [ -z "$API" ] || [ -z "$CURL_CMD" ]
then
  echo "Error: API and CURL_CMD variables must be set before sourcing handle_curl_execution.sh"
  exit 1
fi

# Build full curl command
FULL_CURL_CMD="${CURL_CMD} \"${API}\""

if [ ! -z "$DEBUG" ]
then
  echo "$FULL_CURL_CMD"
  eval $FULL_CURL_CMD
else
  if [ "$USE_JQ" = "true" ]
  then
    eval $FULL_CURL_CMD | jq -r '.'
  else
    eval $FULL_CURL_CMD
  fi
fi