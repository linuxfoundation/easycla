#!/bin/bash
# Typical TOKEN='-' XACL='-' ALL=1 ./utils/run-single-test-remote.sh
# Manually: DEBUG=1 xvfb-run -a npx cypress run --env DEBUG=1
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
