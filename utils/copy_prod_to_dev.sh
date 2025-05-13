#!/bin/bash
# USE_FILE=1

if [ -z "$1" ]
then
  echo "$0: you need to specify table to copy from"
  exit 1
fi
table="${1}"

if [ -z "$2" ]
then
  echo "$0: you need to specify ${table} key column name"
  exit 2
fi
key_column="${2}"

if [ -z "$3" ]
then
  echo "$0: you need to specify ${table} ${key_column} value"
  exit 3
fi
key_value="${3}"

if [ ! -z "$DEBUG" ]
then
  echo "aws --profile lfproduct-prod dynamodb get-item --table-name cla-prod-${table} --key {\"${key_column}\": {\"S\": \"${key_value}\"}} | jq -c .Item"
fi

if [ -z "${USE_FILE}" ]
then
  object=$(aws --profile lfproduct-prod dynamodb get-item --table-name cla-prod-${table} --key "{\"${key_column}\": {\"S\": \"${key_value}\"}}" | jq -c .Item)
  command="aws --profile lfproduct-dev dynamodb put-item --table-name \"cla-dev-${table}\" --item '${object}'"
  if [ ! -z "$DEBUG" ]
  then
    echo "${object}" | jq .
    echo "${command}"
  fi
  eval $command
else
  tmp_file=$(mktemp)
  trap 'rm -f "${tmp_file}"' EXIT
  aws --profile lfproduct-prod dynamodb get-item --table-name cla-prod-${table} --key "{\"${key_column}\": {\"S\": \"${key_value}\"}}" > "${tmp_file}"
  if [ ! -z "$DEBUG" ]
  then
    cat "${tmp_file}" | jq .
    echo "aws --profile lfproduct-dev dynamodb put-item --table-name cla-dev-${table} --cli-input-json file://${tmp_file}"
  fi
  aws --profile lfproduct-dev dynamodb put-item --table-name cla-dev-${table} --cli-input-json "file://${tmp_file}"
  if [ $? -ne 0 ]
  then
    echo "Failed for the following json:"
    cat "${tmp_file}"
  fi
fi
