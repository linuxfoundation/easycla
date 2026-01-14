#!/bin/bash
aws --region us-east-1 --profile lfproduct-dev dynamodb scan --table-name cla-dev-api-log --max-items 100 | jq -r '.Items'
