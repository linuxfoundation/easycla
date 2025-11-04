#!/bin/bash
# GET /company/signing-entity-name
# Getcompanybysigningentityname (authenticated)
# Usage: ./get_company_by_signing_entity_name.sh
# API_URL=http://localhost:5001 ./get_company_by_signing_entity_name.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./get_company_by_signing_entity_name.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle authentication
. ./utils/shared/handle_auth.sh

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v3/company/signing-entity-name"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\" -H \"X-ACL: ${XACL}\" -H \"Authorization: Bearer ${TOKEN}\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
