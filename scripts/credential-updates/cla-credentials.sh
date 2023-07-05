#! /bin/bash

# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# This script updates additional vars needed for the CLA
# projects. Only parameters provided in the list below are updated.

# CLA URLs
CLA_API_BASE='';
CLA_CONTRIBUTOR_BASE='';
CORPORATE_BASE_URL='';
CLA_API_URL=''; # Needed for frontend
CORP_CONSOLE_LINK=''; # Needed for frontend

# CLA AWS Configurations
CLA_BUCKET_LOGO_URL='';
CLA_SIGNATURE_FILES_BUCKET='';
SES_SENDER_EMAIL_ADDRESS='';

# LFID LDAP OAuth2 Credentials
LF_GROUP_CLIENT_ID='';
LF_GROUP_CLIENT_SECRET='';
LF_GROUP_REFRESH_TOKEN='';
LF_GROUP_CLIENT_URL='';

# Auth0 Credentials
AUTH0_DOMAIN='';
AUTH0_CLIENT_ID='';
AUTH0_USERNAME_CLAIM='';
AUTH0_ALGORITHM='';

# DocuSign and Docraptor Credentials
DOCRAPTOR_API_KEY=''
DOCUSIGN_USER_ID=''
DOCUSIGN_PRIVATE_KEY=''
DOCUSIGN_INTEGRATOR_KEY=''
DOCUSIGN_AUTH_SERVER=''

# Github Credentials
GH_APP_ID=''
GH_APP_PUBLIC_LINK=''
GH_APP_WEBHOOK_SECRET=''
GH_APP_PRIVATE_KEY_PATH='' # This is a filename
GH_OAUTH_CLIENT_ID=''
GH_OAUTH_SECRET=''
GH_OAUTH_CLIENT_ID_GO_BACKEND=''
GH_OAUTH_SECRET_GO_BACKEND=''

# SFDC Credentials
INSTANCE_URL=''
USERNAME=''
PASSWORD=''
SECURITY_TOKEN=''
CONSUMER_KEY=''
CONSUMER_SECRET=''

# Needed for go backend
SESSION_STORE_TABLE_NAME=''
ALLOWED_ORIGINS_COMMA_SEPARATED=''

ENV='';
PROFILE='';

if [ -z "$ENV" ]; then
    echo "ERROR: missing environment"
    exit 1
fi

if [ -z "$PROFILE" ]; then
    echo "ERROR: missing profile"
    exit 1
fi

# CLA URLs
if [ -n "$CLA_API_BASE" ]; then
    echo "updating api base URL: $CLA_API_BASE"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-api-base-$ENV" --description "CLA api base url" --value "$CLA_API_BASE" --type "String" --overwrite
fi

if [ -n "$CLA_CONTRIBUTOR_BASE" ]; then
    echo "updating contributor base URL: $CLA_CONTRIBUTOR_BASE"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-contributor-base-$ENV" --description "CLA contributor base url" --value "$CLA_CONTRIBUTOR_BASE" --type "String" --overwrite
fi

if [ -n "$CORPORATE_BASE_URL" ]; then
    echo "updating corporate base URL: $CORPORATE_BASE_URL"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-corporate-base-$ENV" --description "CLA contributor base url" --value "$CORPORATE_BASE_URL" --type "String" --overwrite
fi

# Needed for frontend
if [ -n "$CLA_API_URL" ]; then
    echo "updating API URL: $CLA_API_URL"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-cla-api-url-$ENV" --description "CLA API URL" --value "$CLA_API_URL" --type "String" --overwrite
fi

# Needed for frontend
if [ -n "$CORP_CONSOLE_LINK" ]; then
    echo "updating corp console link: $CORP_CONSOLE_LINK"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-corp-console-link-$ENV" --description "Corporation Console Link" --value "$CORP_CONSOLE_LINK" --type "String" --overwrite
fi

# CLA AWS Configurations
if [ -n "$CLA_BUCKET_LOGO_URL" ]; then
    echo "updating cla logo url: $CLA_BUCKET_LOGO_URL"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-logo-url-$ENV" --description "CLA Logo S3 URL" --value "$CLA_BUCKET_LOGO_URL" --type "String" --overwrite
fi

if [ -n "$CLA_SIGNATURE_FILES_BUCKET" ]; then
    echo "updating cla signature files bucket: $CLA_SIGNATURE_FILES_BUCKET"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-signature-files-bucket-$ENV" --description "CLA signature files bucket" --value "$CLA_SIGNATURE_FILES_BUCKET" --type "String" --overwrite
fi

if [ -n "$SES_SENDER_EMAIL_ADDRESS" ]; then
    echo "updating ses sender email address: $SES_SENDER_EMAIL_ADDRESS"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-ses-sender-email-address-$ENV" --description "SES Sender Email Address" --value "$SES_SENDER_EMAIL_ADDRESS" --type "String" --overwrite
fi

# LFID LDAP OAuth2 Credentials
if [ -n "$LF_GROUP_CLIENT_ID" ]; then
    echo "updating lf group client id: $LF_GROUP_CLIENT_ID"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-lf-group-client-id-$ENV" --description "LFID Group Client ID" --value "$LF_GROUP_CLIENT_ID" --type "String" --overwrite
fi

if [ -n "$LF_GROUP_CLIENT_SECRET" ]; then
    echo "updating lf group client secret: $LF_GROUP_CLIENT_SECRET"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-lf-group-client-secret-$ENV" --description "LFID Group Client Secret" --value "$LF_GROUP_CLIENT_SECRET" --type "String" --overwrite
fi

if [ -n "$LF_GROUP_REFRESH_TOKEN" ]; then
    echo "updating lf group refresh token: $LF_GROUP_REFRESH_TOKEN"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-lf-group-refresh-token-$ENV" --description "LFID Group Refresh Token" --value "$LF_GROUP_REFRESH_TOKEN" --type "String" --overwrite
fi

if [ -n "$LF_GROUP_CLIENT_URL" ]; then
    echo "updating lf group client url: $LF_GROUP_CLIENT_URL"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-lf-group-client-url-$ENV" --description "LFID Group Client URL" --value "$LF_GROUP_CLIENT_URL" --type "String" --overwrite
fi

# Auth0 Credentials
if [ -n "$AUTH0_DOMAIN" ]; then
    echo "updating Auth0 Domain: $AUTH0_DOMAIN"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-auth0-domain-$ENV" --description "Auth0 Domain" --value "$AUTH0_DOMAIN" --type "String" --overwrite
fi

if [ -n "$AUTH0_CLIENT_ID" ]; then
    echo "updating Auth0 Client ID: $AUTH0_CLIENT_ID"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-auth0-clientId-$ENV" --description "Auth0 Client ID" --value "$AUTH0_CLIENT_ID" --type "String" --overwrite
fi

if [ -n "$AUTH0_USERNAME_CLAIM" ]; then
    echo "updating Auth0 Client ID: $AUTH0_USERNAME_CLAIM"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-auth0-username-claim-$ENV" --description "Auth0 username claim" --value "$AUTH0_USERNAME_CLAIM" --type "String" --overwrite
fi

if [ -n "$AUTH0_ALGORITHM" ]; then
    echo "updating Auth0 Client ID: $AUTH0_ALGORITHM"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-auth0-algorithm-$ENV" --description "Auth0 algorithm" --value "$AUTH0_ALGORITHM" --type "String" --overwrite
fi

# DocuSign and Docraptor Credentials
if [ -n "$DOCRAPTOR_API_KEY" ]; then
    echo "updating Docraptor API Key: $DOCRAPTOR_API_KEY"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-doc-raptor-api-key-$ENV" --description "Docraptor API Key" --value "$DOCRAPTOR_API_KEY" --type "String" --overwrite
fi

if [ -n "$DOCUSIGN_USER_ID" ]; then
    echo "updating DocuSign Userid: $DOCUSIGN_USER_ID"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-docusign-user-id-$ENV" --description "DocuSign Userid" --value "$DOCUSIGN_USER_ID" --type "String" --overwrite
fi

if [ -n "$DOCUSIGN_PRIVATE_KEY" ]; then
    echo "updating DocuSign Private key : xxxxx"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-docusign-private-key-$ENV" --description "Docusign Private Key" --value "$DOCUSIGN_PRIVATE_KEY" --type "String" --overwrite
fi

if [ -n "$DOCUSIGN_INTEGRATOR_KEY" ]; then
    echo "updating DocuSign Integrator Key: $DOCUSIGN_INTEGRATOR_KEY"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-docusign-integrator-key-$ENV" --description "Docusign Integrator Key" --value "$DOCUSIGN_INTEGRATOR_KEY" --type "String" --overwrite
fi

if [ -n "$DOCUSIGN_AUTH_SERVER" ]; then
    echo "updating DocuSign Auth Server: $DOCUSIGN_AUTH_SERVER"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-docusign-auth-server-$ENV" --description "DocuSign Auth Server" --value "$DOCUSIGN_AUTH_SERVER" --type "String" --overwrite
fi

# Github Credentials
if [ -n "$GH_APP_ID" ]; then
    echo "updating app ID: $GH_APP_ID"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-gh-app-id-$ENV" --description "Github App ID" --value "$GH_APP_ID" --type "String" --overwrite
fi

if [ -n "$GH_OAUTH_CLIENT_ID" ]; then
    echo "updating oauth client ID: $GH_OAUTH_CLIENT_ID"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-gh-oauth-client-id-$ENV" --description "Github oauth client ID" --value "$GH_OAUTH_CLIENT_ID" --type "String" --overwrite
fi

if [ -n "$GH_APP_PUBLIC_LINK" ]; then
    echo "updating public link: $GH_APP_PUBLIC_LINK"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-gh-app-public-link-$ENV" --description "Github app public link" --value "$GH_APP_PUBLIC_LINK" --type "String" --overwrite
fi

if [ -n "$GH_OAUTH_SECRET" ]; then
    echo "updating oauth secret: $GH_OAUTH_SECRET"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-gh-oauth-secret-$ENV" --description "Github oauth secret" --value "$GH_OAUTH_SECRET" --type "String" --overwrite
fi

if [ -n "$GH_APP_WEBHOOK_SECRET" ]; then
    echo "updating webhook secret: $GH_APP_WEBHOOK_SECRET"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-gh-app-webhook-secret-$ENV" --description "Github webhook secret" --value "$GH_APP_WEBHOOK_SECRET" --type "String" --overwrite
fi

if [ -n "$GH_APP_PRIVATE_KEY_PATH" ]; then
    echo "updating private key: $GH_APP_PRIVATE_KEY_PATH"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-gh-app-private-key-$ENV" --description "Github private key" --value "file://$GH_APP_PRIVATE_KEY_PATH" --type "String" --overwrite
fi

if [ -n "$GH_OAUTH_CLIENT_ID_GO_BACKEND" ]; then
    echo "updating oauth client ID: $GH_OAUTH_CLIENT_ID_GO_BACKEND"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-gh-oauth-client-id-go-backend-$ENV" --description "Github oauth client ID for go backend" --value "$GH_OAUTH_CLIENT_ID_GO_BACKEND" --type "String" --overwrite
fi

if [ -n "$GH_OAUTH_SECRET_GO_BACKEND" ]; then
    echo "updating oauth secret: $GH_OAUTH_SECRET_GO_BACKEND"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-gh-oauth-secret-go-backend-$ENV" --description "Github oauth secret for go backend" --value "$GH_OAUTH_SECRET_GO_BACKEND" --type "String" --overwrite
fi

# SFDC Credentials
if [ -n "$INSTANCE_URL" ]; then
    echo "updating instance url: $INSTANCE_URL"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-sf-instance-url-$ENV" --description "SalesForce instance URL" --value "$INSTANCE_URL" --type "String" --overwrite
fi

if [ -n "$USERNAME" ]; then
    echo "updating username: $USERNAME"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-sf-username-$ENV" --description "SalesForce user name" --value "$USERNAME" --type "String" --overwrite
fi

# The SalesForce API password is the user password concatenated with a security token.
if [ -n "$PASSWORD$SECURITY_TOKEN" ]; then
    echo "updating password: $PASSWORD$SECURITY_TOKEN"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-sf-password-$ENV" --description "SalesForce password. Combined user password and secret token" --value "$PASSWORD$SECURITY_TOKEN" --type "String" --overwrite
fi

if [ -n "$CONSUMER_KEY" ]; then
    echo "updating consumer key: $CONSUMER_KEY"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-sf-consumer-key-$ENV" --description "SalesForce Connected App Consumer Key" --value "$CONSUMER_KEY" --type "String" --overwrite
fi

if [ -n "$CONSUMER_SECRET" ]; then
    echo "updating consumer secret: $CONSUMER_SECRET"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-sf-consumer-secret-$ENV" --description "SalesForce Connected App Consumer Secret" --value "$CONSUMER_SECRET" --type "String" --overwrite
fi

if [ -n "$SESSION_STORE_TABLE_NAME" ]; then
    echo "updating session store table name: $SESSION_STORE_TABLE_NAME"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-session-store-table-$ENV" --description "Dynamo DB Table to store sessions" --value "$SESSION_STORE_TABLE_NAME" --type "String" --overwrite
fi

if [ -n "$ALLOWED_ORIGINS_COMMA_SEPARATED" ]; then
    echo "updating session store table name: $ALLOWED_ORIGINS_COMMA_SEPARATED"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-allowed-origins-$ENV" --description "Allowed origins for CORS" --value "$ALLOWED_ORIGINS_COMMA_SEPARATED" --type "String" --overwrite
fi
