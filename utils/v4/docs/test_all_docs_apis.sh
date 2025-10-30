#!/bin/bash
# Test ALL V4 Documentation API endpoints
# Usage: ./test_all_docs_apis.sh
# API_URL=http://localhost:5001 ./test_all_docs_apis.sh
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service ./test_all_docs_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 Documentation API Endpoints ==="
echo "API_URL: ${API_URL}"
echo ""

echo "1. Testing GET /api-docs (public endpoint)"
echo "   Command: ${SCRIPT_DIR}/get_api_docs.sh"
${SCRIPT_DIR}/get_api_docs.sh
echo ""

echo "2. Testing GET /swagger.json (public endpoint)"
echo "   Command: ${SCRIPT_DIR}/get_swagger_json.sh"
${SCRIPT_DIR}/get_swagger_json.sh
echo ""

echo "=== V4 Documentation API Testing Complete ==="