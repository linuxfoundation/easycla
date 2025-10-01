#!/bin/bash
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
if ( [ -z "$1" ] || [ -z "$2" ] )
then
  echo "Usage: $0 <company_id> <lfid>"
  echo "Example: $0 f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc lgryglicki"
  exit 1
fi
if [ -n "$DEBUG" ]
then
  echo "aws --profile lfproduct-${STAGE} dynamodb update-item --table-name cla-${STAGE}-companies --key '{\"company_id\":{\"S\":\"${1}\"}}' --update-expression 'ADD company_acl :u' --expression-attribute-values '{\":u\":{\"SS\":[\"${2}\"]}}'"
fi

aws --profile "lfproduct-${STAGE}" dynamodb update-item --table-name "cla-${STAGE}-companies" --key "{\"company_id\":{\"S\":\"${1}\"}}" --update-expression "ADD company_acl :u" --expression-attribute-values "{\":u\":{\"SS\":[\"${2}\"]}}"
