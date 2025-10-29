#!/bin/bash
aws --profile lfproduct-dev --region us-east-1 ssm put-parameter \
         --name "/cla-auth0-username-claim-dev" \
         --value "https://sso.linuxfoundation.org/claims/username" \
         --type "String" \
         --overwrite
# was: https://sso.linuxfoundation.org/claims/username
# needed for e2e tests: http://lfx.dev/claims/username
./utils/get_username_claim.sh
