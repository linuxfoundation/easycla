#!/bin/bash
if [ -z "$1" ]
then
  echo "$0: you need to specify lfid as a 1st parameter, for example: 'lgryglicki'"
  exit 1
fi
snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select object_construct('user_id', user_id, 'data', data) from fivetran_ingest.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_USERS where lower(data:lf_username) = lower('${1}')" | jq -r '.'
