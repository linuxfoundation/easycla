#!/bin/bash
# Test ALL V3 API endpoints using curl scripts
# Usage: ./test_all_v3_apis.sh
# API_URL=http://localhost:5001 TOKEN="$(cat ./token.secret)" ./test_all_v3_apis.sh  
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org TOKEN="$(cat ./token.secret)" ./test_all_v3_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Set defaults for environment variables
if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

# For authenticated endpoints (users API)
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
echo "Testing ALL V3 API Endpoints"
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

# Test Organization APIs (public)
echo "======================================="
echo "3. ORGANIZATION APIs (Public)"
echo "======================================="
${SCRIPT_DIR}/organization/test_all_organization_apis.sh
echo ""

# Test Version APIs (public)
echo "======================================="
echo "4. VERSION APIs (Public)"
echo "======================================="
${SCRIPT_DIR}/version/test_all_version_apis.sh
echo ""

# Test Users APIs (authenticated)
if [ ! -z "$TOKEN" ] && [ ! -z "$XACL" ]; then
  echo "======================================="
  echo "5. USERS APIs (Authenticated)"
  echo "======================================="
  ${SCRIPT_DIR}/users/test_all_users_apis.sh
  echo ""
else
  echo "======================================="
  echo "5. USERS APIs (Authenticated) - SKIPPED"
  echo "======================================="
  echo "TOKEN and/or XACL not provided - skipping authenticated endpoints"
  echo "To test users APIs, provide TOKEN and XACL:"
  echo "  TOKEN=\"\$(cat ./token.secret)\" XACL=\"\$(cat ./x-acl.secret)\" $0"
  echo ""
fi

echo "======================================="
echo "V3 API Testing Complete!"
echo "======================================="
echo ""
echo "Summary:"
echo "✓ Documentation APIs (2 endpoints) - Public"
echo "✓ Health APIs (1 endpoint) - Public" 
echo "✓ Organization APIs (1 endpoint) - Public"
echo "✓ Version APIs (1 endpoint) - Public"
if [ ! -z "$TOKEN" ] && [ ! -z "$XACL" ]; then
  echo "✓ Users APIs (7+ endpoints) - Authenticated"
  echo ""
  echo "Total: 12+ endpoints tested"
else
  echo "⚠ Users APIs (7+ endpoints) - Skipped (no auth)"
  echo ""
  echo "Total: 5 endpoints tested (7+ skipped)"
fi