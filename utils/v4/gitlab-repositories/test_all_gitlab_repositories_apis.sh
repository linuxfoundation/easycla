#!/bin/bash
# Test ALL V4 GitLab Repositories API endpoints
# Usage: ./test_all_gitlab_repositories_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_gitlab_repositories_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 GitLab Repositories API Endpoints ==="
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
  echo "1. Testing GET /project/{projectSFID}/gitlab/repositories (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_project_gitlab_repositories.sh <project-sfid>"
  echo "   [Skipping - requires valid project SFID]"
  echo ""

  echo "2. Testing PUT /project/{projectSFID}/gitlab/repositories (enroll/unenroll)"
  echo "   Command: ${SCRIPT_DIR}/enroll_unenroll_gitlab_repositories.sh <project-sfid> <cla-group-id> <action> <repo-ids>"
  echo "   [Skipping - requires valid parameters]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test GitLab repositories APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 GitLab Repositories API Testing Complete ==="