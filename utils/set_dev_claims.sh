#!/bin/bash
# This is needed for V3 CLA Auth0 setup
aws --profile lfproduct-dev --region us-east-1 ssm put-parameter --name "/cla-auth0-username-claim-cli-dev" --value "http://lfx.dev/claims/username" --type "String" --overwrite
aws --profile lfproduct-dev --region us-east-1 ssm put-parameter --name "/cla-auth0-email-claim-cli-dev" --value "http://lfx.dev/claims/email" --type "String" --overwrite
aws --profile lfproduct-dev --region us-east-1 ssm put-parameter --name "/cla-auth0-name-claim-cli-dev" --value "http://lfx.dev/claims/username" --type "String" --overwrite
./utils/get_dev_claims.sh
