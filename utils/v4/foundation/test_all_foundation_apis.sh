#!/bin/bash
# Test ALL V4 Foundation API endpoints
# Usage: ./test_all_foundation_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_foundation_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 Foundation API Endpoints ==="
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
  echo "1. Testing GET /foundation-mapping (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_foundation_mapping.sh"
  ${SCRIPT_DIR}/get_foundation_mapping.sh
  echo ""

  echo "2. Testing GET /foundation-mapping?foundationSFID={id} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_foundation_mapping.sh <foundation-sfid>"
  echo "   [Skipping - requires valid foundation SFID]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test foundation APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 Foundation API Testing Complete ==="