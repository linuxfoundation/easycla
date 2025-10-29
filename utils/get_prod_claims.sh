#!/bin/bash
aws --profile lfproduct-prod --region us-east-1 ssm get-parameter --name "/cla-auth0-username-claim-prod" --query "Parameter.Value" --output text
aws --profile lfproduct-prod --region us-east-1 ssm get-parameter --name "/cla-auth0-username-claim-cli-prod" --query "Parameter.Value" --output text
aws --profile lfproduct-prod --region us-east-1 ssm get-parameter --name "/cla-auth0-email-claim-cli-prod" --query "Parameter.Value" --output text
aws --profile lfproduct-prod --region us-east-1 ssm get-parameter --name "/cla-auth0-name-claim-cli-prod" --query "Parameter.Value" --output text
aws --profile lfproduct-prod --region us-east-2 ssm get-parameter --name "/cla-auth0-username-claim-prod" --query "Parameter.Value" --output text
aws --profile lfproduct-prod --region us-east-2 ssm get-parameter --name "/cla-auth0-username-claim-cli-prod" --query "Parameter.Value" --output text
aws --profile lfproduct-prod --region us-east-2 ssm get-parameter --name "/cla-auth0-email-claim-cli-prod" --query "Parameter.Value" --output text
ews --profile lfproduct-prod --region us-east-2 ssm get-parameter --name "/cla-auth0-name-claim-cli-prod" --query "Parameter.Value" --output text
