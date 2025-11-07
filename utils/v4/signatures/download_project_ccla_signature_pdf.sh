#!/bin/bash
# GET /signatures/project/{claGroupID}/ccla/{signatureID}/pdf
# Download CCLA signature PDF (authenticated)
# Usage: ./download_project_ccla_signature_pdf.sh <cla_group_id> <signature_id>
# Example: ./download_project_ccla_signature_pdf.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e sig123
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./download_project_ccla_signature_pdf.sh <cla_group_id> <signature_id>

if [ -z "$1" ] || [ -z "$2" ]
then
  echo "$0: you need to specify cla_group_id and signature_id as parameters"
  echo "Usage: $0 <cla_group_id> <signature_id>"
  echo "Example: $0 d9428888-122b-4b20-8c4a-0c9a1a6f9b8e sig123"
  exit 1
fi

export cla_group_id="$1"
export signature_id="$2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/signatures/project/${cla_group_id}/ccla/${signature_id}/pdf"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=false
. ./utils/shared/handle_curl_execution.sh