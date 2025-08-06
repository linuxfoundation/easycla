#!/bin/bash
# PROD=1
# DEBUG=1
# FUNC=lower
# OP=in
# COND='complex expression'
# OUT='col1 col 2 col3'
# example:
# DEBUG=1 PROD=1 FUNC=lower ./utils/lookup_sf.sh projects_cla_groups project_sfid cla_group_name "lower('onap')"
# OUT='project_sfid project_name foundation_sfid' ./utils/lookup_sf.sh projects_cla_groups project_sfid foundation_sfid "'a09P000000DsCE5IAN'"

if [ -z "$PROD" ]
then
  schema='dynamodb_product_us_east1_dev'
  prefix='cla_dev_'
else
  schema='dynamodb_product_us_east_1'
  prefix='cla_prod_'
fi
if [ -z "$1" ]
then
  echo "you need to specify table to query as a 1st parameter, for example 'signatures'"
  echo "possible tables include: approvals, ccla_whitelist_requests, cla_manager_requests, companies, company_invites, events, gerrit_instances, github_orgs, gitlab_orgs, metrics, projects, projects_cla_groups, repositories, session_store, signatures, store, user_permissions, users"
  exit 1
fi
if [ -z "$2" ]
then
  echo "you need to specify table's primary key column name as a 2nd parameter, see example row from the table to determine one:"
  snowsql $(cat ./snowflake.secret) -o friendly=false -o header=true -o timing=false -o output_format=plain -q "select * from fivetran_ingest.${schema}.${prefix}${1} limit 1"
  exit 2
fi
if [ -z "$COND" ]
then
  if [ -z "$3" ]
  then
    echo "$0: you need to specify $1 column as a 3rd parameter, see available columns to choose one:"
    snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select object_construct('${2}', ${2}, 'data', data) from fivetran_ingest.${schema}.${prefix}${1} limit 1"
    exit 3
  fi
  if [ -z "$4" ]
  then
    echo "$0: you need to specify ${1} ${3} value as a 4th parameter"
    echo "$0: if that column is strinf then you need to specify like this: \"'value'\""
    exit 4
  fi
  if [ "$3" = "$2" ]
  then
    col="$2"
  else
    col="data:${3}"
  fi
  if [ ! -z "$FUNC" ]
  then
    col="${FUNC}(${col})"
  fi
  if [ "$OP" = "in" ]
  then
    cond="array_contains(${4}::variant, ${col})"
  else
    cond="${col} = ${4}"
  fi
else
  cond="${COND}"
fi
if [ ! -z "$DEBUG" ]
then
  echo "snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q \"select object_construct('${2}', ${2}, 'data', data) from fivetran_ingest.${schema}.${prefix}${1} where ${cond}\""
fi
if [ ! -z "$OUT" ]
then
  cols=($OUT)
  n=0
  for c in "${cols[@]}"
  do
    if [ "$c" = "$2" ]
    then
      cc=".${c}"
    else
      cc=".data.${c}"
    fi
    if [ -z "${jqq}" ]
    then
      jqq="${cc}"
    else
      jqq="${jqq},${cc}"
    fi
    n=$((n + 1))
  done
  if [[ $n -gt 1 ]]
  then
    jqq="[${jqq}]"
  fi
  snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select object_construct('${2}', ${2}, 'data', data) from fivetran_ingest.${schema}.${prefix}${1} where ${cond}" | jq -r "${jqq}"
else
  snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select object_construct('${2}', ${2}, 'data', data) from fivetran_ingest.${schema}.${prefix}${1} where ${cond}" | jq -r '.'
fi
