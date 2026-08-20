#!/bin/bash
# STAGE=dev|prod
# DEBUG=1
# example update: STAGE=prod DEBUG=1 ./utils/update_company_is_sanctioned.sh "0ca30016-6457-466c-bc41-a09560c1f9bf" false
if [ -z "$STAGE" ]
then
  export STAGE=dev
fi
if [ -z "$1" ]
then
  echo "$0: you need to specify company_id, for example: '0ca30016-6457-466c-bc41-a09560c1f9bf', '10bde6b1-3061-4972-9c6a-17dd9a175a5c'"
  exit 1
fi
if [ -z "$2" ]
then
  echo "$0: you need to value: true|false"
  exit 2
fi
# Setting the flag stamps sanctioned_date, as the backends do; clearing it keeps the stored date.
upd_expr="SET is_sanctioned = :val"
values="{\":val\":{\"BOOL\":${2}}}"
if [ "$2" = "true" ]
then
  upd_expr="SET is_sanctioned = :val, sanctioned_date = :now"
  values="{\":val\":{\"BOOL\":true},\":now\":{\"S\":\"$(date -u '+%Y-%m-%dT%H:%M:%S.%6N+0000')\"}}"
fi
if [ ! -z "$DEBUG" ]
then
  echo aws --profile "lfproduct-$STAGE" dynamodb update-item --table-name "cla-${STAGE}-companies" --key "{\"company_id\":{\"S\":\"${1}\"}}" --update-expression "\"${upd_expr}\"" --expression-attribute-values "$values"
fi
aws --profile "lfproduct-$STAGE" dynamodb update-item --table-name "cla-${STAGE}-companies" --key "{\"company_id\":{\"S\":\"${1}\"}}" --update-expression "$upd_expr" --expression-attribute-values "$values"
