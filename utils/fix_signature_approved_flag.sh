#!/bin/bash
if [ -z "$1" ]
then
  echo "$0: please specify signature_id as a 1st parameter"
  exit 1
fi
signature_id="${1}"

secret="$(cat ./snowflake.secret)"

signature_data=$(snowsql $secret -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select data:signature_reference_id, data:signature_project_id, data:signature_user_ccla_company_id from FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_SIGNATURES where signature_id = '${signature_id}' and data:signature_reference_type = 'user' and data:signature_type = 'cla' and data:signature_approved = false")
signature_data="${signature_data//\"/}"
ary=($signature_data)
user_id="${ary[0]}"
project_id="${ary[1]}"
company_id="${ary[2]}"
if [ ! -z "$DEBUG" ]
then
  echo "signature ${signature_id} data: user: ${user_id}, project: ${project_id}, company: ${company_id}"
fi

ccla_data=$(snowsql $secret -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select distinct data:domain_whitelist from FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_SIGNATURES where data:signature_project_id = '${project_id}' and data:signature_reference_id = '${company_id}' and data:signature_reference_type = 'company' and data:signature_type = 'ccla' and data:domain_whitelist is not null" | jq -rc)
ccla_data="${ccla_data//\[/}"
ccla_data="${ccla_data//]/}"
ccla_data="${ccla_data//\"/}"
ccla_data="${ccla_data//,/ }"
if [ ! -z "$DEBUG" ]
then
  echo "ccla domain: ${ccla_data}"
fi

user_data=$(snowsql $secret -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select distinct data:user_emails from FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_USERS where user_id = '${user_id}'" | jq -rc)
user_data="${user_data//\[/}"
user_data="${user_data//]/}"
user_data="${user_data//\"/}"
user_data="${user_data//,/ }"
user_email=$(snowsql $secret -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select data:lf_email from FIVETRAN_INGEST.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_USERS where user_id = '${user_id}'")
user_email="${user_email//\"/}"
if ( [ ! -z "${user_email}" ] && [ ! "${user_email}" = "NULL" ] )
then
  user_data="${user_data} ${user_email}"
fi
if [ ! -z "$DEBUG" ]
then
  echo "user's emails: $user_data"
fi

for email in $user_data
do
  usr_domain="${email#*@}"
  usr_domain=$(echo "$usr_domain" | xargs)
  for ccla_domain in $ccla_data
  do
    ccla_domain=$(echo "$ccla_domain" | xargs)
    if [ "${usr_domain}" = "${ccla_domain}" ]
    then
      echo "${signature_id}"
      exit 0
    fi
  done
done
