#!/bin/bash
if [ -z "$1" ]
then
  echo "$0: you need to specify which column as a 1st parameter, for example: 'cla_group_name'"
  echo "possible columns include: project_sfid, cla_group_id, cla_group_name, date_created, date_modified, foundation_name, foundation_sfid, note, project_external_id, project_name, repositories_count, version"
  exit 1
fi
if [ -z "$2" ]
then
  echo "$0: you need to specify '${1}' value as a 2nd parameter, for example: 'onap'"
  exit 2
fi
snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select object_construct('project_sfid', project_sfid, 'data', data) from fivetran_ingest.DYNAMODB_PRODUCT_US_EAST_1.CLA_PROD_PROJECTS_CLA_GROUPS where lower(data:${1}) = lower('${2}')" | jq -r '.'
