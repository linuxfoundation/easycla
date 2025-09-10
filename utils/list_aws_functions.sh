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
aws lambda list-functions --region "$REGION" --profile "lfproduct-${STAGE}" --query 'Functions[*].FunctionName' --output text |
  tr '\t' '\n' |
  sort -u |
  if [ -n "$FILTER" ]; then
    grep -i "$FILTER"
  else
    cat
  fi
