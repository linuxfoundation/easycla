#!/bin/bash
# Check if user is CLA Manager Designee (public endpoint)
# Usage: ./is_cla_manager_designee.sh <company_sfid> <user_lfid> <cla_group_id>
# Example: ./is_cla_manager_designee.sh a01234567890123456 john.doe e1234567-abcd-4567-8901-234567890abc

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify company_sfid, user_lfid, and cla_group_id as parameters"
  echo "Usage: $0 <company_sfid> <user_lfid> <cla_group_id>"
  echo "Example: $0 a01234567890123456 john.doe e1234567-abcd-4567-8901-234567890abc"
  exit 1
fi

export company_sfid="$1"
export user_lfid="$2"
export cla_group_id="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/company/${company_sfid}/user/${user_lfid}/claGroupID/${cla_group_id}/is-cla-manager-designee"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh