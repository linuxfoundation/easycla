#!/bin/bash
if [ -z "$1" ]
then
  echo "$0: you need to specify which column as a 1st parameter, for example: 'lf_username'"
  echo "possible columns include: user_id, admin, date_created, date_modified, lf_email, lf_username, note, user_company_id, user_emails, user_external_id, user_github_id, user_github_username, user_name, version"
  exit 1
fi
if [ -z "$2" ]
then
  echo "$0: you need to specify '${1}' value as a 2nd parameter, for example: 'lgryglicki'"
  exit 2
fi
snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select object_construct('user_id', user_id, 'data', data) from fivetran_ingest.dynamodb_product_us_east_1.cla_prod_users where lower(data:${1}) = lower('${2}')" | jq -r '.'
