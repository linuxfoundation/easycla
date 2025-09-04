#!/bin/bash
LOCAL=1 XACL="$(cat ./x-acl.secret)" TOKEN="$(cat ./token.secret)" ALLOW_FAIL=1 ./utils/run-single-test.sh "$@"
