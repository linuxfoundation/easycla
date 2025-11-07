#!/bin/bash
# List metrics for a specific company and project (authenticated)
# Usage: ./list_company_project_metrics.sh <company_id> <project_sfid>
# Example: ./list_company_project_metrics.sh a1b86c26-d8e8-4fd8-9f8d-5c723d5dac9f a09P000000DsNH2IAN
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./list_company_project_metrics.sh <company_id> <project_sfid>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify company_id and project_sfid as parameters"
  echo "Usage: $0 <company_id> <project_sfid>"
  echo "Example: $0 a1b86c26-d8e8-4fd8-9f8d-5c723d5dac9f a09P000000DsNH2IAN"
  exit 1
fi
export company_id="$1"
export project_sfid="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/metrics/company/${company_id}/project/${project_sfid}"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh