#!/bin/bash
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
if ( [ -z "$1" ] || [ -z "$2" ] )
then
  echo "Usage: $0 <signature_id> <lfid>"
  echo "Example: $0 24d71d78-2266-46c0-a795-698419e50bdb lgryglicki"
  exit 1
fi
if [ -n "$DEBUG" ]
then
  echo "aws --profile lfproduct-${STAGE} dynamodb update-item --table-name cla-${STAGE}-signatures --key '{\"signature_id\":{\"S\":\"${1}\"}}' --update-expression 'ADD signature_acl :u' --expression-attribute-values '{\":u\":{\"SS\":[\"${2}\"]}}'"
fi

aws --profile "lfproduct-${STAGE}" dynamodb update-item --table-name "cla-${STAGE}-signatures" --key "{\"signature_id\":{\"S\":\"${1}\"}}" --update-expression "ADD signature_acl :u" --expression-attribute-values "{\":u\":{\"SS\":[\"${2}\"]}}"
