#!/bin/bash
set -euo pipefail
export AWS_PAGER=""
for f in $(./utils/list_aws_functions.sh)
do
  echo "${f}:"
  ./utils/get_function_config.sh "${f}"
done
