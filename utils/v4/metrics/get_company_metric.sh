#!/bin/bash
# Get metrics of a specific company (authenticated)
# Usage: ./get_company_metric.sh <company_id>
# Example: ./get_company_metric.sh a1b86c26-d8e8-4fd8-9f8d-5c723d5dac9f
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_company_metric.sh <company_id>
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./get_company_metric.sh <company_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify company_id as a 1st parameter"
  echo "Usage: $0 <company_id>"
  echo "Example: $0 a1b86c26-d8e8-4fd8-9f8d-5c723d5dac9f"
  exit 1
fi
export company_id="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/metrics/company/${company_id}"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh