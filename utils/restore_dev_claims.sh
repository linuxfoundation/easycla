#!/bin/bash
# This is needed for V3 CLA Auth0 setup
aws --profile lfproduct-dev --region us-east-1 ssm put-parameter --name "/cla-auth0-username-claim-dev" --value "https://sso.linuxfoundation.org/claims/username" --type "String" --overwrite
aws --profile lfproduct-dev --region us-east-1 ssm delete-parameter --name "/cla-auth0-username-claim-cli-dev"
aws --profile lfproduct-dev --region us-east-1 ssm delete-parameter --name "/cla-auth0-email-claim-cli-dev"
aws --profile lfproduct-dev --region us-east-1 ssm delete-parameter --name "/cla-auth0-name-claim-cli-dev"
aws --profile lfproduct-dev --region us-east-2 ssm put-parameter --name "/cla-auth0-username-claim-dev" --value "https://sso.linuxfoundation.org/claims/username" --type "String" --overwrite
aws --profile lfproduct-dev --region us-east-2 ssm delete-parameter --name "/cla-auth0-username-claim-cli-dev"
aws --profile lfproduct-dev --region us-east-2 ssm delete-parameter --name "/cla-auth0-email-claim-cli-dev"
aws --profile lfproduct-dev --region us-east-2 ssm delete-parameter --name "/cla-auth0-name-claim-cli-dev"
./utils/get_dev_claims.sh
