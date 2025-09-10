#!/bin/bash
set -euo pipefail
export AWS_PAGER=""
if [ -z "$REGION" ]
then
  REGION="us-east-2"
fi
if [ -z "$STAGE" ]
then
  STAGE="dev"
fi
FILTER=${FILTER:-}
aws ssm describe-parameters --region "$REGION" --profile "lfproduct-${STAGE}" --query 'Parameters[].Name' --output text |
  tr '\t' '\n' |
  sort -u |
  if [ -n "$FILTER" ]; then
    grep -i "$FILTER"
  else
    cat
  fi
