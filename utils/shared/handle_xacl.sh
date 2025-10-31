#!/bin/bash
# Shared handler for XACL environment variable
# Reads from x-acl.secret if XACL not already set  
# Exits with error code 2 if XACL cannot be obtained
# Usage: . ./utils/shared/handle_xacl.sh

if [ -z "$XACL" ]
then
  XACL="$(cat ./x-acl.secret 2>/dev/null || echo '')"
fi

if [ -z "$XACL" ]
then
  echo "$0: XACL not specified and unable to obtain one"
  exit 2
fi