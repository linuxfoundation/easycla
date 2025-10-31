#!/bin/bash
# Shared handler for TOKEN environment variable  
# Reads from token.secret if TOKEN not already set
# Exits with error code 1 if token cannot be obtained
# Usage: . ./utils/shared/handle_token.sh

if [ -z "$TOKEN" ]
then
  TOKEN="$(cat ./token.secret 2>/dev/null || echo '')"
fi

if [ -z "$TOKEN" ]
then
  echo "$0: TOKEN not specified and unable to obtain one"
  exit 1
fi