#!/bin/bash
# SINGLE=1 will print each parameter one by one
for f in $(./utils/list_ssm_parameters.sh)
do
  if [ -z "$SINGLE" ]
  then
    if [ -z "${params}" ]
    then
      params="${f}"
    else
      params="${params} ${f}"
    fi
  else
    echo "${f}:"
    ./utils/get_ssm_value.sh "${f}"
  fi
done
if [ -z "${SINGLE}" ]
then
  echo "${params}:"
  ./utils/get_ssms_values.sh ${params}
fi
