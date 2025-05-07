#!/bin/bash
# STAGE=prod
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

declare -A template_projects
declare -A projects

JQ='[.[] | select((.document_major_version != null) and (.document_minor_version != null) and (.document_creation_date != null))] | sort_by((.document_major_version | tonumber), (.document_minor_version | tonumber), .document_creation_date) | .[-1].document_s3_url'

data=$(snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select distinct object_construct('project_id', project_id, 'project_name', data:project_name) from ${TABLE} order by 1 limit 10000" | jq -s -r '.')
# echo $data
while read -r project_id project_name
do
  template=$(snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select data:project_individual_documents from ${TABLE} where project_id = '${project_id}'" | jq -r "${JQ}")
  if ( [ -z "${template}" ] || [ "${template}" = "null" ] )
  then
    continue
  fi
  echo "ICLA ${project_id}"
  if [ -n "${template_projects[$template]}" ]
  then
    template_projects["$template"]+=";${project_id}"
  else
    template_projects["$template"]="${project_id}"
  fi
  projects["$project_id"]="${project_name}"
done < <(echo "${data}" | jq -r -c '.[] | "\(.project_id) \(.project_name)"')

while read -r project_id project_name
do
  template=$(snowsql $(cat ./snowflake.secret) -o friendly=false -o header=false -o timing=false -o output_format=plain -q "select data:project_corporate_documents from ${TABLE} where project_id = '${project_id}'" | jq -r "${JQ}")
  if ( [ -z "${template}" ] || [ "${template}" = "null" ] )
  then
    continue
  fi
  echo "CCLA ${project_id}"
  if [ -n "${template_projects[$template]}" ]
  then
    template_projects["$template"]+=";${project_id}"
  else
    template_projects["$template"]="${project_id}"
  fi
  projects["$project_id"]="${project_name}"
done < <(echo "${data}" | jq -r -c '.[] | "\(.project_id) \(.project_name)"')

# for template in "${!template_projects[@]}"
for template in $(printf "%s\n" "${!template_projects[@]}" | sort)
do
  projs="${template_projects[$template]}"
  names=""
  np=0
  IFS=';' read -ra idary <<< "${projs}"
  for id in "${idary[@]}"; do
    if [ -z "${names}" ]
    then
      names="${projects[$id]} (${id})"
    else
      names+=";${projects[$id]} (${id})"
    fi
    ((np++))
  done
  IFS=';' read -ra ary <<< "${names}"
  sorted_projects=$(printf "%s\n" "${ary[@]}" | sort | paste -sd ',' | sed 's/),/), /g')
  echo "Template: ${template} used by ${np} project(s): ${sorted_projects}"
done
