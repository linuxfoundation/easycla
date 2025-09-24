#!/bin/bash
# requires "npm i -D @cypress/grep"
# ALLOW_FAIL=1 - means don't fail early on HTTP 4xx, 5xx errors
# TOKEN=xxxx
# XACL=xxxx
# DEBUG=1 - print the command to be executed
# LOCAL=1 - run against local backend (http://localhost:8001)
# V=1|2|3|4 - defaults to 4 (meaning V4 APIs)
# ALL=1 - run all tests
# Example: ALL='' V=4 ALLOW_FAIL=1 LOCAL=1 DEBUG=1 TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./utils/run-single-test.sh github-repositories 'Get GitHub branch protection for given repository - Record should Returns 200 Response'

if [ -z "${V}" ]
then
  export V=4
fi

if [ -z "$1" ]
then
  if [ ! -z "${ALL}" ]
  then
    echo "Running all tests"
  else
    echo "Usage: $0 <test-file-name-without-extension> [test-name-regexp]"
    echo "Example (v4 APIs groups): $0 cla-group, cla-manager, company, docs, events, foundation, github-organizations, github-repositories, githubActivity, gitlab-organizations, gitlab-repositories, health, metrics, projects, signatures, version"
    exit 1
  fi
fi

if [ ! -z "${2}" ]
then
  export CYPRESS_grep="${2}"
fi

if [ -z "${ALL}" ]
then
  CMD="xvfb-run -a npx cypress run --spec cypress/e2e/v${V}/${1}.cy.ts"
else
  CMD="xvfb-run -a npx cypress run"
fi

ENV_ARGS=""

if [ ! -z "${ALLOW_FAIL}" ]; then
  ENV_ARGS="${ENV_ARGS:+$ENV_ARGS,}ALLOW_FAIL=1"
fi

if [ ! -z "${LOCAL}" ]; then
  ENV_ARGS="${ENV_ARGS:+$ENV_ARGS,}LOCAL=1"
fi

if [ ! -z "${DEBUG}" ]; then
  ENV_ARGS="${ENV_ARGS:+$ENV_ARGS,}DEBUG=1"
fi

if [ ! -z "${TOKEN}" ]; then
  ENV_ARGS="${ENV_ARGS:+$ENV_ARGS,}TOKEN=${TOKEN}"
fi

if [ ! -z "${XACL}" ]; then
  ENV_ARGS="${ENV_ARGS:+$ENV_ARGS,}XACL=${XACL}"
fi

if [ ! -z "${ENV_ARGS}" ]; then
  CMD="${CMD} --env ${ENV_ARGS}"
fi

npx prettier --write cypress/e2e/* cypress/support/* cypress/appConfig/* cypress.config.ts
if [ ! -z "${DEBUG}" ]
then
  if [ ! -z "${CYPRESS_grep}" ]
  then
    echo "Running: CYPRESS_grep='${CYPRESS_grep}' ${CMD}"
  else
    echo "Running: ${CMD}"
  fi
fi

eval "${CMD}"

