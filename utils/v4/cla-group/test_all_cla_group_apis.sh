#!/bin/bash
# Test ALL V4 CLA Group API endpoints
# Usage: ./test_all_cla_group_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_cla_group_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 CLA Group API Endpoints ==="
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
  echo "1. Testing GET /foundation/{foundationSFID}/cla-groups (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_foundation_cla_groups.sh <foundation-sfid>"
  echo "   [Skipping - requires valid foundation SFID]"
  echo ""

  echo "2. Testing POST /cla-group (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/create_cla_group.sh <foundation-sfid> <project-sfid> <name> <description>"
  echo "   [Skipping - requires valid foundation and project SFIDs]"
  echo ""

  echo "3. Testing PUT /cla-group/{claGroupID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/update_cla_group.sh <cla-group-id> <name> <description>"
  echo "   [Skipping - requires valid CLA group ID]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test CLA group APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 CLA Group API Testing Complete ==="