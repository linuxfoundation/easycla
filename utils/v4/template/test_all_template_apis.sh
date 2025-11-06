#!/bin/bash
# Test ALL V4 Template API endpoints
# Usage: ./test_all_template_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_template_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 Template API Endpoints ==="
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
  echo "1. Testing GET /template (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_template.sh"
  ${SCRIPT_DIR}/get_template.sh
  echo ""

  echo "2. Testing POST /clagroup/{claGroupID}/template (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/post_cla_group_template.sh <cla-group-id> <name> <content>"
  echo "   [Skipping - requires valid parameters]"
  echo ""

  echo "3. Testing POST /template/preview (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/post_template_preview.sh <content>"
  echo "   [Skipping - requires template content]"
  echo ""

  echo "4. Testing GET /template/{claGroupID}/preview (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_template_preview.sh <cla-group-id>"
  echo "   [Skipping - requires valid CLA group ID]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test template APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 Template API Testing Complete ==="