#!/bin/bash
# Test ALL V4 GitLab Organizations API endpoints
# Usage: ./test_all_gitlab_organizations_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_gitlab_organizations_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 GitLab Organizations API Endpoints ==="
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

echo "1. Testing GET /gitlab/group/{gitLabGroupID}/members (public)"
echo "   Command: ${SCRIPT_DIR}/get_gitlab_group_members.sh <gitlab-group-id>"
echo "   [Skipping - requires valid GitLab group ID]"
echo ""

if [ ! -z "$TOKEN" ] && [ ! -z "$XACL" ]; then
  echo "2. Testing GET /project/{projectSFID}/gitlab/organizations (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_project_gitlab_organizations.sh <project-sfid>"
  echo "   [Skipping - requires valid project SFID]"
  echo ""

  echo "3. Testing PUT /project/{projectSFID}/gitlab/group/{gitLabGroupID}/config (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/update_gitlab_group_config.sh <project-sfid> <group-id> <auto> <cla-group> <branch>"
  echo "   [Skipping - requires valid parameters]"
  echo ""

  echo "4. Testing POST /project/{projectSFID}/gitlab/organizations (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/add_gitlab_organization.sh <project-sfid> <group-id> <org-path> <auto> <cla-group> <branch>"
  echo "   [Skipping - requires valid parameters]"
  echo ""

  echo "5. Testing DELETE /project/{projectSFID}/gitlab/organization (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/delete_gitlab_organization.sh <project-sfid> <org-path>"
  echo "   [Skipping - requires valid parameters]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test GitLab organizations APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 GitLab Organizations API Testing Complete ==="