#!/bin/bash
# PROD=1
# LIM=10
# DEBUG=1
if [ -z "$LIM" ]
then
  lim=1
else
  lim=$LIM
fi
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
  echo "$0: required argument - list of columns to check for uniqueness on $1, example: 'lf_username user_company_id'"
  exit 2
fi
query="select"
ary=($2)
n=0
for c in "${ary[@]}"
do
  if [ -z "${cols}" ]
  then
    cols="data:${c}"
    cond="where ${cols} is not null"
  else
    cols="${cols}, data:${c}"
    cond="${cond} and data:${c} is not null"
  fi
done
query="${query} ${cols}, count(*) as cnt from fivetran_ingest.${schema}.${prefix}${1} ${cond} group by all"
query="select i.* from (${query}) i where i.cnt > 1 order by i.cnt desc limit ${lim}"
if [ ! -z "$DEBUG" ]
then
  echo "query: ${query}"
fi
snowsql $(cat ./snowflake.secret) -o friendly=false -o header=true -o timing=false -o output_format=plain -q "${query}"
