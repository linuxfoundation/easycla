#!/bin/bash
# Test all V3 Documentation API endpoints using curl scripts
# Usage: ./test_all_docs_apis.sh
# API_URL=http://localhost:5001 ./test_all_docs_apis.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./test_all_docs_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

export API_URL

echo "=== Testing V3 Documentation API Endpoints ==="
echo "API_URL: ${API_URL}"
echo ""

# Test 1: GET /api-docs (public endpoint)
echo "1. Testing GET /api-docs (public endpoint)"
echo "   Command: ${SCRIPT_DIR}/get_api_docs.sh"
${SCRIPT_DIR}/get_api_docs.sh | head -20
echo "   [Output truncated for readability]"
echo ""

# Test 2: GET /swagger.json (public endpoint)
echo "2. Testing GET /swagger.json (public endpoint)"
echo "   Command: ${SCRIPT_DIR}/get_swagger_json.sh"
${SCRIPT_DIR}/get_swagger_json.sh | head -10
echo "   [Output truncated for readability]"
echo ""

echo "=== V3 Documentation API Testing Complete ==="