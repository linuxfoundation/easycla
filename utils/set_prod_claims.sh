#!/bin/bash
# This is needed for V3 CLA Auth0 setup
aws --profile lfproduct-prod --region us-east-1 ssm put-parameter --name "/cla-auth0-username-claim-cli-prod" --value "http://lfx.prod/claims/username" --type "String" --overwrite
aws --profile lfproduct-prod --region us-east-1 ssm put-parameter --name "/cla-auth0-email-claim-cli-prod" --value "http://lfx.prod/claims/email" --type "String" --overwrite
aws --profile lfproduct-prod --region us-east-1 ssm put-parameter --name "/cla-auth0-name-claim-cli-prod" --value "http://lfx.prod/claims/username" --type "String" --overwrite
aws --profile lfproduct-prod --region us-east-2 ssm put-parameter --name "/cla-auth0-username-claim-cli-prod" --value "http://lfx.prod/claims/username" --type "String" --overwrite
aws --profile lfproduct-prod --region us-east-2 ssm put-parameter --name "/cla-auth0-email-claim-cli-prod" --value "http://lfx.prod/claims/email" --type "String" --overwrite
aws --profile lfproduct-prod --region us-east-2 ssm put-parameter --name "/cla-auth0-name-claim-cli-prod" --value "http://lfx.prod/claims/username" --type "String" --overwrite
./utils/get_prod_claims.sh
