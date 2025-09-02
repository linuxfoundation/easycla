#!/bin/bash
# requires "npm i -D @cypress/grep"
# ALLOW_FAIL=1 - means dont' fail early on HTTP 4xx, 5xx errors
# Example: ALLOW_FAIL=1 ./run-single-test.sh github-repositories 'Get GitHub branch protection for given repository - Record should Returns 200 Response'

if [ -z "$1" ]
then
  echo "Usage: $0 <test-file-name-without-extension> [test-name-regexp]"
  echo "Example: $0 cla-group, cla-manager, company, docs, events, foundation, github-organizations, github-repositories, githubActivity, gitlab-organizations, gitlab-repositories, health, metrics, projects, signatures, version"
  exit 1
fi

if [ ! -z "${2}" ]
then
  export CYPRESS_grep="${2}"
fi

if [ ! -z "${ALLOW_FAIL}" ]
then
  xvfb-run -a npx cypress run --env ALLOW_FAIL=1 --spec "cypress/e2e/${1}.cy.ts"
else
  xvfb-run -a npx cypress run --spec "cypress/e2e/${1}.cy.ts"
fi
