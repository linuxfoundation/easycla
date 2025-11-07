#!/bin/bash
# Download CCLA signatures as PDF for a project (authenticated)
# Usage: ./download_project_ccla_pdfs.sh <project_id>
# Example: ./download_project_ccla_pdfs.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./download_project_ccla_pdfs.sh <project_id>

if [ -z "$1" ]
then
  echo "$0: you need to specify project_id as a parameter"
  echo "Usage: $0 <project_id>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e"
  exit 1
fi

export project_id="$1"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/signatures/project/${project_id}/ccla/pdfs"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=false
. ./utils/shared/handle_curl_execution.sh