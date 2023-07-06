#! /bin/bash

# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# This script updates the DocuSign parameters for a given environment. Only
# parameters provided in the list below are updated.

# The Docraptor API Key
DOCRAPTOR_API_KEY=''

# The DocuSign Username
DOCUSIGN_USERNAME=''

# The DocuSign Password
DOCUSIGN_PASSWORD=''

# The DocuSign Integrator Key
DOCUSIGN_INTEGRATOR_KEY=''

# The DocuSign Root URL
DOCUSIGN_ROOT_URL=''

ENV=''
PROFILE=''

if [ -z "$ENV" ]; then
    echo "ERROR: missing environment"
    exit 1
fi

if [ -z "$PROFILE" ]; then
    echo "ERROR: missing profile"
    exit 1
fi

if [ -n "$DOCRAPTOR_API_KEY" ]; then
    echo "updating Docraptor API Key: $DOCRAPTOR_API_KEY"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-doc-raptor-api-key-$ENV" --description "Docraptor API Key" --value "$DOCRAPTOR_API_KEY" --type "String" --overwrite
fi

if [ -n "$DOCUSIGN_USERNAME" ]; then
    echo "updating DocuSign Username: $DOCUSIGN_USERNAME"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-docusign-username-$ENV" --description "DocuSign Username" --value "$DOCUSIGN_USERNAME" --type "String" --overwrite
fi

if [ -n "$DOCUSIGN_PASSWORD" ]; then
    echo "updating DocuSign Password: $DOCUSIGN_PASSWORD"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-docusign-password-$ENV" --description "Docusign Password" --value "$DOCUSIGN_PASSWORD" --type "String" --overwrite
fi

if [ -n "$DOCUSIGN_INTEGRATOR_KEY" ]; then
    echo "updating DocuSign Integrator Key: $DOCUSIGN_INTEGRATOR_KEY"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-docusign-integrator-key-$ENV" --description "Docusign Integrator Key" --value "$DOCUSIGN_INTEGRATOR_KEY" --type "String" --overwrite
fi

if [ -n "$DOCUSIGN_ROOT_URL" ]; then
    echo "updating DocuSign Root Url: $DOCUSIGN_ROOT_URL"
    aws ssm put-parameter --profile $PROFILE --region us-east-1 --name "cla-docusign-root-url-$ENV" --description "DocuSign Root Url" --value "$DOCUSIGN_ROOT_URL" --type "String" --overwrite
fi
