#!/bin/bash
# add: | jq -r '.[].signature_acl'
aws --profile lfproduct-dev dynamodb scan --table-name cla-dev-signatures --max-items 20 | jq -r '.Items'
