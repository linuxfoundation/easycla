#!/bin/bash
# Test ALL V4 Metrics API endpoints
# Usage: ./test_all_metrics_apis.sh
# TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_metrics_apis.sh
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service TOKEN="$(cat ./token.secret)" XACL="$(cat ./x-acl.secret)" ./test_all_metrics_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 Metrics API Endpoints ==="
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
  echo "1. Testing GET /metrics/cla-manager-distribution (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_cla_manager_distribution.sh"
  ${SCRIPT_DIR}/get_cla_manager_distribution.sh
  echo ""

  echo "2. Testing GET /metrics/total-count (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_total_count.sh"
  ${SCRIPT_DIR}/get_total_count.sh
  echo ""

  echo "3. Testing GET /metrics/top-companies (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_top_companies.sh"
  ${SCRIPT_DIR}/get_top_companies.sh
  echo ""

  echo "4. Testing GET /metrics/top-projects (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_top_projects.sh"
  ${SCRIPT_DIR}/get_top_projects.sh
  echo ""

  echo "5. Testing GET /metrics/project (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/list_project_metrics.sh"
  ${SCRIPT_DIR}/list_project_metrics.sh
  echo ""

  echo "6. Testing GET /metrics/company/{companyID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_company_metric.sh <example-company-id>"
  echo "   [Skipping - requires valid company ID]"
  echo ""

  echo "7. Testing GET /metrics/project/{projectID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/get_project_metric.sh <example-project-id>"
  echo "   [Skipping - requires valid project ID]"
  echo ""

  echo "8. Testing GET /metrics/company/{companyID}/project/{projectSFID} (authenticated)"
  echo "   Command: ${SCRIPT_DIR}/list_company_project_metrics.sh <company-id> <project-sfid>"
  echo "   [Skipping - requires valid company ID and project SFID]"
  echo ""
else
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test metrics APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "=== V4 Metrics API Testing Complete ==="