#!/bin/bash
# Test ALL V4 API endpoints using curl scripts
# Usage: ./test_all_v4_apis.sh
# API_URL=http://localhost:5001 TOKEN="$(cat ./token.secret)" ./test_all_v4_apis.sh  
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service TOKEN="$(cat ./token.secret)" ./test_all_v4_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/shared/handle_api_url.sh

# For authenticated endpoints (metrics API) - handle optionally
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
  echo ""
  echo "Total: 12+ endpoints tested"
else
  echo "⚠ Metrics APIs (8+ endpoints) - Skipped (no auth)"
  echo ""
  echo "Total: 4 endpoints tested (8+ skipped)"
fi