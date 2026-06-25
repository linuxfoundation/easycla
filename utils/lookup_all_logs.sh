#!/bin/bash
set -euo pipefail
if ( [ -z "${1:-}" ] || [ -z "${2:-}" ] || [ -z "${3:-}" ] )
then
  echo "usage:"
  echo "  $0 '2 hours ago' '1 second ago' 'text'"
  echo "  if 'text' = '---' then it returns all logs"
  exit 1
fi
if [ -z "${STAGE:-}" ]
then
  export STAGE=dev
fi

# Fail fast (set -euo pipefail above): any lookup helper failure (aws/jq error, exit 3/4)
# aborts here instead of running on and ending with a successful `ls` that would hide it.
REGION=us-east-1 DEBUG=1 DTFROM="${1}" DTTO="${2}" ./utils/search_aws_log_group.sh 'githubactivity' "${3}" > githubactivity.log
REGION=us-east-1 DEBUG=1 DTFROM="${1}" DTTO="${2}" ./utils/search_aws_log_group.sh 'apiv1' "${3}" > v1.log
REGION=us-east-1 DEBUG=1 DTFROM="${1}" DTTO="${2}" ./utils/search_aws_log_group.sh 'apiv2' "${3}" > v2.log
REGION=us-east-1 DEBUG=1 DTFROM="${1}" DTTO="${2}" ./utils/search_aws_log_group.sh 'api-v3-lambda' "${3}" > v3.log
REGION=us-east-2 DEBUG=1 DTFROM="${1}" DTTO="${2}" ./utils/search_aws_log_group.sh 'cla-backend-go-api-v4-lambda' "${3}" > v4.log
ls -l githubactivity.log v?.log
