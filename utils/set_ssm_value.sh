#!/bin/bash

if [ $# -lt 2 ]
then
  echo "Usage: $0 <ssm-parameter-name> <value> [type]"
  echo "Example: STAGE=dev REGION=us-east-1 $0 cla-dd-site-dev dev String"
  exit 1
fi

set -euo pipefail
export AWS_PAGER=""

PARAM_NAME="$1"
PARAM_VALUE="$2"
PARAM_TYPE="${3:-String}"   # Default to String if not provided

if [ -z "${REGION:-}" ]
then
  REGION="us-east-2"
fi

if [ -z "${STAGE:-}" ]
then
  STAGE="dev"
fi

echo "Setting SSM parameter:"
echo "  Name:   ${PARAM_NAME}"
echo "  Value:  ${PARAM_VALUE}"
echo "  Type:   ${PARAM_TYPE}"
echo "  Region: ${REGION}"
echo "  Stage:  ${STAGE}"
echo

aws ssm put-parameter \
  --region "${REGION}" \
  --profile "lfproduct-${STAGE}" \
  --name "${PARAM_NAME}" \
  --value "${PARAM_VALUE}" \
  --type "${PARAM_TYPE}" \
  --overwrite

echo "Done."

