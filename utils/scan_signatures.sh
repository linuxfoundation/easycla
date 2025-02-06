#!/bin/bash
# add: | jq -r '.[].signature_acl'
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
aws --profile "lfproduct-${STAGE}" dynamodb scan --table-name "cla-${STAGE}-signatures" --max-items 100 | jq -r '.Items'
