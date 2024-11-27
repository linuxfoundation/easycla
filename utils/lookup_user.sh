#!/bin/bash
aws --profile lfproduct-dev dynamodb query --table-name cla-dev-users --index-name github-username-index --key-condition-expression "user_github_username = :name" --expression-attribute-values  '{":name":{"S":"lukaszgryglicki"}}'
