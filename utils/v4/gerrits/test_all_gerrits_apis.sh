#!/bin/bash
# Test ALL V4 Gerrits API endpoints
# Usage: ./test_all_gerrits_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_gerrits_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 Gerrits API Endpoints ==="
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
  echo "1. Testing GET /gerrit/repos (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_gerrit_repos.sh"
  ${SCRIPT_DIR}/get_gerrit_repos.sh
  echo ""

  echo "2. Testing GET /cla-group/{claGroupID}/project/{projectSFID}/gerrits (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_project_gerrits.sh <cla-group-id> <project-sfid>"
  echo "   [Skipping - requires valid parameters]"
  echo ""

  echo "3. Testing POST /cla-group/{claGroupID}/project/{projectSFID}/gerrits (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/add_project_gerrit.sh <cla-group-id> <project-sfid> <name> <url> <icla-group> <ccla-group>"
  echo "   [Skipping - requires valid parameters]"
  echo ""

  echo "4. Testing DELETE /cla-group/{claGroupID}/project/{projectSFID}/gerrits/{gerritID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/delete_project_gerrit.sh <cla-group-id> <project-sfid> <gerrit-id>"
  echo "   [Skipping - requires valid parameters]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test Gerrits APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 Gerrits API Testing Complete ==="