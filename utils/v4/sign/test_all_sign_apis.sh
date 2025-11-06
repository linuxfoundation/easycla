#!/bin/bash
# Test ALL V4 Sign API endpoints
# Usage: ./test_all_sign_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_sign_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 Sign API Endpoints ==="
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
  echo "1. Testing GET /user/{userID}/active-signature (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_user_active_signature.sh <user-id>"
  echo "   [Skipping - requires valid user ID]"
  echo ""

  echo "2. Testing POST /clear-cache (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/post_clear_cache.sh"
  echo "   [Skipping - cache operation]"
  echo ""

  echo "3. Testing POST /request-corporate-signature (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/post_request_corporate_signature.sh <proj> <comp> <url>"
  echo "   [Skipping - requires valid parameters]"
  echo ""

  echo "4. Testing POST /request-individual-signature (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/post_request_individual_signature.sh <proj> <user> <url>"
  echo "   [Skipping - requires valid parameters]"
  echo ""

  echo "5. Testing POST /signed/corporate/{project_id}/{company_id} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/post_signed_corporate.sh <proj> <comp> <data>"
  echo "   [Skipping - signature submission]"
  echo ""

  echo "6. Testing POST /signed/gerrit/individual/{user_id} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/post_signed_gerrit_individual.sh <user> <data>"
  echo "   [Skipping - signature submission]"
  echo ""

  echo "7. Testing POST /signed/gitlab/individual/{user_id}/{organization_id}/{gitlab_repository_id}/{merge_request_id} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/post_signed_gitlab_individual.sh <user> <org> <repo> <mr> <data>"
  echo "   [Skipping - signature submission]"
  echo ""

  echo "8. Testing POST /signed/individual/{installation_id}/{github_repository_id}/{change_request_id} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/post_signed_individual.sh <inst> <repo> <pr> <data>"
  echo "   [Skipping - signature submission]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test sign APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 Sign API Testing Complete ==="