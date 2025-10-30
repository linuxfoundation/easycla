#!/bin/bash
if [ -z "${TOKEN}" ]                                                                                                                                                                          
then                                                                                                                                                                                          
  export TOKEN="$(cat ./token.secret)"
fi
if [ -z "${XACL}" ] 
then
  export XACL="$(cat ./x-acl.secret)" 
fi
# export ALLOW_FAIL=1
DEBUG=1 ./utils/run-single-test.sh "$@"
