#!/bin/bash
# Shared handler for API_URL environment variable for V3 APIs
# Sets API_URL to remote dev server if REMOTE=1, otherwise defaults to localhost
# Usage: . ./utils/v3/shared/handle_api_url.sh

if [ -z "$API_URL" ]
then
  if [ "$REMOTE" = "1" ]
  then
    export API_URL="https://api.lfcla.dev.platform.linuxfoundation.org"
  else
    export API_URL="http://localhost:5001"
  fi
fi