#!/bin/bash
aws --profile lfproduct-dev --region us-east-1 ssm get-parameter --name "/cla-auth0-username-claim-dev" --query "Parameter.Value" --output text
aws --profile lfproduct-dev --region us-east-1 ssm get-parameter --name "/cla-auth0-username-claim-cli-dev" --query "Parameter.Value" --output text
aws --profile lfproduct-dev --region us-east-1 ssm get-parameter --name "/cla-auth0-email-claim-cli-dev" --query "Parameter.Value" --output text
aws --profile lfproduct-dev --region us-east-1 ssm get-parameter --name "/cla-auth0-name-claim-cli-dev" --query "Parameter.Value" --output text
