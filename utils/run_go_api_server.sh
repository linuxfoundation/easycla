#!/bin/bash
make build-linux && PORT=5001 AUTH0_USERNAME_CLAIM_CLI='http://lfx.dev/claims/username' AUTH0_EMAIL_CLAIM_CLI='http://lfx.dev/claims/email' AUTH0_NAME_CLAIM_CLI='http://lfx.dev/claims/username' ./bin/cla
