#!/bin/bash
if [ -z "$1" ]
then
  echo "$0: you need to specify gerrit_id as a 1st parameter, for example: 'ae9ae8aa-c44c-4181-9b15-4a98f188b711'"
  exit 1
fi

if [ -z "$STAGE" ]
then
  STAGE=dev
fi
aws --profile "lfproduct-${STAGE}" dynamodb delete-item --table-name "cla-${STAGE}-gerrit-instances" --key "{\"gerrit_id\":{\"S\":\"${1}\"}}"
