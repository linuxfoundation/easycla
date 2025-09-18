#!/bin/bash
ALLOW_FAIL=1 LOCAL=1 DEBUG=1 TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./utils/run-single-test.sh "$@"
