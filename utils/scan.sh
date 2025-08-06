#!/bin/bash
# STAGE=prod ./utils/scan.sh signatures signature_reference_id 29df5aa6-a396-4543-968c-f6e15f011d43
# ALL=1
# DEBUG=1 ALL=1 ./utils/scan.sh gerrit-instances
if [ -z "$STAGE" ]
then
  STAGE=dev
fi
if [ "$STAGE" = "dev" ]
then
  schema='dynamodb_product_us_east1_dev'
  prefix='cla_dev_'
fi
if [ "$STAGE" = "prod" ]
then
  schema='dynamodb_product_us_east_1'
  prefix='cla_prod_'
fi
if [ -z "$1" ]
then
  echo "you need to specify table to query as a 1st parameter, for example 'signatures'"
  echo "possible tables include: approvals, ccla_whitelist_requests, cla_manager_requests, companies, company_invites, events, gerrit_instances, github_orgs, gitlab_orgs, metrics, projects, projects_cla_groups, repositories, session_store, signatures, store, user_permissions, users"
  exit 1
fi
if ( [ -z "$2" ] && [ -z "$ALL" ] )
then
  echo "$0: you need to specify '$1' table's column as a 2nd argument, see columns:"
  snowsql $(cat ./snowflake.secret) -o friendly=false -o header=true -o timing=false -o output_format=plain -q "select * from fivetran_ingest.${schema}.${prefix}${1} limit 1"
  exit 2
fi
if ( [ -z "$3" ] && [ -z "$ALL" ] )
then
  echo "$0: you need to specify '$1' table '$2' column value to search for"
  exit 3
fi
if ( [ -z "$2" ] || [ -z "$3" ] )
then
  aws --profile "lfproduct-${STAGE}" dynamodb scan --table-name "cla-${STAGE}-${1}" --max-items 100 | jq -r '.Items'
else
  aws --profile "lfproduct-${STAGE}" dynamodb scan --table-name "cla-${STAGE}-${1}" --filter-expression "contains(${2},:v)" --expression-attribute-values "{\":v\":{\"S\":\"${3}\"}}" --max-items 100 | jq -r '.Items'
fi
