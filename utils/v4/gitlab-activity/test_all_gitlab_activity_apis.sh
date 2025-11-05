#!/bin/bash
# Test ALL V4 GitLab Activity API endpoints
# Usage: ./test_all_gitlab_activity_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_gitlab_activity_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 GitLab Activity API Endpoints ==="
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
  echo "1. Testing GET /gitlab/oauth/callback (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_gitlab_oauth_callback.sh [code] [state]"
  echo "   [Skipping - OAuth callback requires specific parameters]"
  echo ""

  echo "2. Testing GET /gitlab/user/oauth/callback (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_gitlab_user_oauth_callback.sh [code] [state]"
  echo "   [Skipping - OAuth callback requires specific parameters]"
  echo ""

  echo "3. Testing POST /gitlab/activity (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/post_gitlab_activity.sh <event> <action> [payload]"
  echo "   [Skipping - webhook requires specific event data]"
  echo ""

  echo "4. Testing POST /gitlab/trigger (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/post_gitlab_trigger.sh <trigger-data>"
  echo "   [Skipping - trigger requires specific data]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test GitLab activity APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 GitLab Activity API Testing Complete ==="