#!/bin/bash
cd .. && source setenv.sh && cd cla-backend && AUTH0_USERNAME_CLAIM_CLI='http://lfx.dev/claims/username' AUTH0_EMAIL_CLAIM_CLI='http://lfx.dev/claims/email' AUTH0_NAME_CLAIM_CLI='http://lfx.dev/claims/username' yarn serve:ext 1>python-api.log 2>python-api.err
# cd .. && source setenv.sh && cd cla-backend && yarn serve:ext 1>python-api.log 2>python-api.err
