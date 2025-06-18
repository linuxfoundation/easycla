#!/bin/bash

rm -rf /tmp/aws
cp -R ~/.aws /tmp/.aws

dev_arn="$(cat ./product-contractors-role.dev.secret)"
data="$(aws sts assume-role --role-arn ${dev_arn} --profile lfproduct --role-session-name lfproduct-dev-session)"
export AWS_ACCESS_KEY_ID="$(echo "${data}" | jq -r '.Credentials.AccessKeyId')"
export AWS_SECRET_ACCESS_KEY="$(echo "${data}" | jq -r '.Credentials.SecretAccessKey')"
export AWS_SESSION_TOKEN="$(echo "${data}" | jq -r '.Credentials.SessionToken')"
export AWS_SECURITY_TOKEN="$(echo "${data}" | jq -r '.Credentials.SessionToken')"
export GITHUB_OAUTH_TOKEN="$(cat /etc/github/oauth)"
export DOCUSIGN_INTEGRATOR_KEY="$(cat ./DOCUSIGN_INTEGRATOR_KEY.secret)"
export DOCUSIGN_USER_ID="$(cat ./DOCUSIGN_USER_ID.secret)"
export DOCUSIGN_AUTH_SERVER="$(cat ./DOCUSIGN_AUTH_SERVER.secret)"
export DOCUSIGN_ROOT_URL="$(cat ./DOCUSIGN_ROOT_URL.secret)"
export DOCUSIGN_ACCOUNT_ID="$(cat ./DOCUSIGN_ACCOUNT_ID.secret)"

export AWS_SDK_LOAD_CONFIG=true
export AWS_PROFILE='lfproduct-dev'
export AWS_REGION='us-east-1'
export AWS_DEFAULT_REGION='us-east-1'
export DYNAMODB_AWS_REGION='us-east-1'
export REGION='us-east-1'

export PRODUCT_DOMAIN='dev.lfcla.com'
export ROOT_DOMAIN='lfcla.dev.platform.linuxfoundation.org'
export PORT='5000'
export STAGE='dev'
# export STAGE='local'
export GH_ORG_VALIDATION=false
export DISABLE_LOCAL_PERMISSION_CHECKS=true
export COMPANY_USER_VALIDATION=false
export CLA_SIGNATURE_FILES_BUCKET=cla-signature-files-dev
