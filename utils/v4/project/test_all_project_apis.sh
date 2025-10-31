#!/bin/bash
# Test ALL V4 Project API endpoints
# Usage: ./test_all_project_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_project_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 Project API Endpoints ==="
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
  echo "1. Testing GET /project (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_projects.sh"
  ${SCRIPT_DIR}/get_projects.sh
  echo ""

  echo "2. Testing GET /project/enabled/{foundationSFID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_cla_enabled_projects.sh <foundation-sfid>"
  echo "   [Skipping - requires valid foundation SFID]"
  echo ""

  echo "3. Testing GET /project/external/{externalID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_project_by_external_id.sh <external-id>"
  echo "   [Skipping - requires valid external ID]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test project APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 Project API Testing Complete ==="