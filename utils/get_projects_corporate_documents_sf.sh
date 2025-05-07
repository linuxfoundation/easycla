#!/bin/bash
# STAGE=prod
# JQ='.[] | {document_major_version, document_minor_version, document_file_id, document_name, document_creation_date, document_s3_url}'
# JQ='sort_by(.document_major_version, .document_minor_version, .document_creation_date) | .[-1]'
# JQ='sort_by(.document_major_version, .document_minor_version, .document_creation_date) | .[-1] | {document_major_version, document_minor_version, document_file_id, document_name, document_creation_date, document_s3_url}'
# d8cead54-92b7-48c5-a2c8-b1e295e8f7f1 - prod CNCF project ID
if [ -z "$1" ]
then
  echo "$0: you need to specify project_id as a 1st argument, example: 'd8cead54-92b7-48c5-a2c8-b1e295e8f7f1'"
  exit 1
fi
if [ -z "${STAGE}" ]
then
  export STAGE=dev
fi
if [ "${STAGE}" = "prod" ]
then
  export TABLE="fivetran_ingest.dynamodb_product_us_east_1.cla_prod_projects"
else
  export TABLE="fivetran_ingest.dynamodb_product_us_east1_dev.cla_dev_projects"
fi
if [ -z "${JQ}" ]
then
  snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select data:project_corporate_documents from ${TABLE} where project_id = '${1}'" | jq -r '.'
else
  snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select data:project_corporate_documents from ${TABLE} where project_id = '${1}'" | jq -r "${JQ}"
fi
