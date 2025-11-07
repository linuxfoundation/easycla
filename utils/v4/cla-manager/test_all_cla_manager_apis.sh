#!/bin/bash
# Test ALL V4 CLA Manager API endpoints
# Usage: ./test_all_cla_manager_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_cla_manager_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 CLA Manager API Endpoints ==="
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

echo "1. Testing GET /company/{companySFID}/user/{userLFID}/claGroupID/{claGroupID}/is-cla-manager-designee (public)"
echo "   Command: ${SCRIPT_DIR}/is_cla_manager_designee.sh <company-sfid> <user-lfid> <cla-group-id>"
echo "   [Skipping - requires valid IDs]"
echo ""

if [ ! -z "$TOKEN" ] && [ ! -z "$XACL" ]; then
  echo "2. Testing POST /company/{companyID}/project/{projectSFID}/cla-manager (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/add_cla_manager.sh <company-id> <project-sfid> <first-name> <last-name> <email>"
  echo "   [Skipping - requires valid IDs]"
  echo ""

  echo "3. Testing DELETE /company/{companyID}/project/{projectSFID}/cla-manager/{userLFID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/delete_cla_manager.sh <company-id> <project-sfid> <user-lfid>"
  echo "   [Skipping - requires valid IDs]"
  echo ""

  echo "4. Testing POST /company/{companyID}/project/{projectSFID}/cla-manager/requests (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/add_cla_manager_request.sh <company-id> <project-sfid> <full-name>"
  echo "   [Skipping - requires valid IDs]"
  echo ""

  echo "5. Testing POST /company/{companyID}/claGroup/{claGroupID}/cla-manager-designee (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/assign_cla_manager_designee.sh <company-id> <cla-group-id> <email>"
  echo "   [Skipping - requires valid IDs]"
  echo ""

  echo "6. Testing POST /notify-cla-managers (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/notify_cla_managers.sh <params>"
  echo "   [Skipping - requires valid parameters]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test CLA manager APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 CLA Manager API Testing Complete ==="