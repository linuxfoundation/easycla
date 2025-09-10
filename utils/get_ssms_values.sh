#!/bin/bash
set -euo pipefail
export AWS_PAGER=""
if [ -z "$1" ]
then
  echo "Usage: $0 <ssm-parameter-name> [...]"
  exit 1
fi
if [ -z "$REGION" ]
then
  REGION="us-east-2"
fi
if [ -z "$STAGE" ]
then
  STAGE="dev"
fi
# aws ssm get-parameters --region "${REGION}" --profile "lfproduct-${STAGE}" --names $@ --with-decryption
names=( "$@" )
batch_size=10
total=${#names[@]}
i=0
while [ $i -lt $total ]
do
  batch=( "${names[@]:$i:$batch_size}" )
  echo "${batch[*]}:"
  aws ssm get-parameters --region "$REGION" --profile "lfproduct-${STAGE}" --with-decryption --names "${batch[@]}"
  i=$(( i + batch_size ))
done
