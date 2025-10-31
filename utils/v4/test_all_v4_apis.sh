#!/bin/bash
# Test ALL V4 API endpoints using curl scripts
# Usage: ./test_all_v4_apis.sh
# API_URL=http://localhost:5001 TOKEN="$(cat ./token.secret)" ./test_all_v4_apis.sh  
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service TOKEN="$(cat ./token.secret)" ./test_all_v4_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/shared/handle_api_url.sh

# For authenticated endpoints - handle optionally
if [ -z "$TOKEN" ]
then
  TOKEN="$(cat ./token.secret 2>/dev/null || echo '')"
fi

if [ -z "$XACL" ]
then
  XACL="$(cat ./x-acl.secret 2>/dev/null || echo '')"
fi

export TOKEN
export XACL
export API_URL

echo "======================================="
echo "Testing ALL V4 API Endpoints"
echo "======================================="
echo "API_URL: ${API_URL}"
if [ ! -z "$TOKEN" ]; then
  echo "TOKEN: ${TOKEN:0:20}..."
else
  echo "TOKEN: [not provided - authenticated endpoints will be skipped]"
fi
if [ ! -z "$XACL" ]; then
  echo "XACL: ${XACL:0:20}..."
else
  echo "XACL: [not provided - authenticated endpoints will be skipped]"
fi
echo ""

# Test Documentation APIs (public)
echo "======================================="
echo "1. DOCUMENTATION APIs (Public)"
echo "======================================="
${SCRIPT_DIR}/docs/test_all_docs_apis.sh
echo ""

# Test Health APIs (public)
echo "======================================="
echo "2. HEALTH APIs (Public)"
echo "======================================="
${SCRIPT_DIR}/health/test_all_health_apis.sh
echo ""

# Test Version APIs (public)
echo "======================================="
echo "3. VERSION APIs (Public)"
echo "======================================="
${SCRIPT_DIR}/version/test_all_version_apis.sh
echo ""

# Test Metrics APIs (authenticated)
echo "======================================="
echo "4. METRICS APIs (Authenticated)"
echo "======================================="
${SCRIPT_DIR}/metrics/test_all_metrics_apis.sh
echo ""

# Test CLA Group APIs (authenticated)
echo "======================================="
echo "5. CLA GROUP APIs (Authenticated)"
echo "======================================="
${SCRIPT_DIR}/cla-group/test_all_cla_group_apis.sh
echo ""

# Test Company APIs (authenticated)
echo "======================================="
echo "6. COMPANY APIs (Authenticated)"
echo "======================================="
${SCRIPT_DIR}/company/test_all_company_apis.sh
echo ""

# Test Events APIs (authenticated)
echo "======================================="
echo "7. EVENTS APIs (Authenticated)"
echo "======================================="
${SCRIPT_DIR}/events/test_all_events_apis.sh
echo ""

# Test Foundation APIs (authenticated)
echo "======================================="
echo "8. FOUNDATION APIs (Authenticated)"
echo "======================================="
${SCRIPT_DIR}/foundation/test_all_foundation_apis.sh
echo ""

# Test Project APIs (authenticated)
echo "======================================="
echo "9. PROJECT APIs (Authenticated)"
echo "======================================="
${SCRIPT_DIR}/project/test_all_project_apis.sh
echo ""

# Test GitHub Organizations APIs (authenticated)
echo "======================================="
echo "10. GITHUB ORGANIZATIONS APIs (Authenticated)"
echo "======================================="
${SCRIPT_DIR}/github-organizations/test_all_github_organizations_apis.sh
echo ""

# Test GitHub Repositories APIs (authenticated)
echo "======================================="
echo "11. GITHUB REPOSITORIES APIs (Authenticated)"
echo "======================================="
${SCRIPT_DIR}/github-repositories/test_all_github_repositories_apis.sh
echo ""

# Test Signatures APIs (authenticated)
echo "======================================="
echo "12. SIGNATURES APIs (Authenticated)"
echo "======================================="
${SCRIPT_DIR}/signatures/test_all_signatures_apis.sh
echo ""

echo "======================================="
echo "V4 API Testing Complete!"
echo "======================================="
echo ""
echo "Summary:"
echo "✓ Documentation APIs (2 endpoints) - Public"
echo "✓ Health APIs (1 endpoint) - Public" 
echo "✓ Version APIs (1 endpoint) - Public"
if [ ! -z "$TOKEN" ] && [ ! -z "$XACL" ]; then
  echo "✓ Metrics APIs (8+ endpoints) - Authenticated"
  echo "✓ CLA Group APIs (3+ endpoints) - Authenticated"
  echo "✓ Company APIs (4+ endpoints) - Authenticated"
  echo "✓ Events APIs (3+ endpoints) - Authenticated"
  echo "✓ Foundation APIs (2+ endpoints) - Authenticated"
  echo "✓ Project APIs (3+ endpoints) - Authenticated"
  echo "✓ GitHub Organizations APIs (1+ endpoints) - Authenticated"
  echo "✓ GitHub Repositories APIs (1+ endpoints) - Authenticated"
  echo "✓ Signatures APIs (1+ endpoints) - Authenticated"
  echo ""
  echo "Total: 29+ endpoints tested across 12 API categories"
else
  echo "⚠ Metrics APIs (8+ endpoints) - Skipped (no auth)"
  echo "⚠ CLA Group APIs (3+ endpoints) - Skipped (no auth)"
  echo "⚠ Company APIs (4+ endpoints) - Skipped (no auth)"
  echo "⚠ Events APIs (3+ endpoints) - Skipped (no auth)"
  echo "⚠ Foundation APIs (2+ endpoints) - Skipped (no auth)"
  echo "⚠ Project APIs (3+ endpoints) - Skipped (no auth)"
  echo "⚠ GitHub Organizations APIs (1+ endpoints) - Skipped (no auth)"
  echo "⚠ GitHub Repositories APIs (1+ endpoints) - Skipped (no auth)"
  echo "⚠ Signatures APIs (1+ endpoints) - Skipped (no auth)"
  echo ""
  echo "Total: 4 endpoints tested (25+ skipped due to no auth)"
fi