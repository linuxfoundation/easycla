#!/bin/bash
# STAGE=prod ./utils/lookup_store_sf.sh 'active_signature:f6a0ed33-917b-4336-b4c3-145f0f357274'
if [ -z "$1" ]
then
  echo "$0: you need to specify key as a 1st argument"
  echo "example: 'active_signature:564e571e-12d7-4857-abd4-898939accdd7'"
  exit 1
fi
if [ -z "${STAGE}" ]
then
  export STAGE=dev
fi
if [ "${STAGE}" = "prod" ]
then
  snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select data from fivetran_ingest.dynamodb_product_us_east_1.cla_prod_store where key = '${1}'" | jq -r '.'
else
  snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select data from fivetran_ingest.dynamodb_product_us_east1_dev.cla_dev_store where key = '${1}'" | jq -r '.'
fi
