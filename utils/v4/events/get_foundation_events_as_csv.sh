#!/bin/bash
# GET /events/foundation/{foundationSFID}/csv
# Getfoundationeventsascsv (authenticated)
# Example: ./get_foundation_events_as_csv.sh param1
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_foundation_events_as_csv.sh <foundation_sfid>

if [ -z "$1" ]
then
  echo "$0: you need to specify foundation_sfid as parameter"
  echo "Usage: $0 <foundation_sfid>"
  echo "Example: $0 param1"
  exit 1
fi

export foundation_sfid="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/events/foundation/${foundation_sfid}/csv"
CURL_CMD="curl -s -XGET -H "Content-Type: application/json" -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
