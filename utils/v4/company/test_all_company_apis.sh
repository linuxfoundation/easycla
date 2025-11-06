#!/bin/bash
# Test ALL V4 Company API endpoints
# Usage: ./test_all_company_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_company_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 Company API Endpoints ==="
echo "API_URL: ${API_URL}"
echo ""

# For authenticated endpoints - handle optionally
if [ -z "$TOKEN" ]
then
  TOKEN="$(cat ./token.secret 2>/dev/null || echo '')"
fi

if [ -z "$XACL" ]
then
  XACL="$(cat ./x-acl.secret 2>/dev/null || echo '')"
fi

if [ ! -z "$TOKEN" ] && [ ! -z "$XACL" ]; then
  echo "1. Testing GET /company/{companyID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_company_by_id.sh <company-id>"
  echo "   [Skipping - requires valid company ID]"
  echo ""

  echo "2. Testing GET /company/external/{companySFID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_company_by_external_id.sh <company-sfid>"
  echo "   [Skipping - requires valid company SFID]"
  echo ""

  echo "3. Testing GET /company/name/{companyName} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_company_by_name.sh \"<company-name>\""
  echo "   [Skipping - requires valid company name]"
  echo ""

  echo "4. Testing GET /company/{companyID}/project/{projectSFID}/cla-managers (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_company_cla_managers.sh <company-id> <project-sfid>"
  echo "   [Skipping - requires valid company ID and project SFID]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test company APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 Company API Testing Complete ==="