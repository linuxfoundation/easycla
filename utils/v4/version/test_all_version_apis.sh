#!/bin/bash
# Test ALL V4 Version API endpoints
# Usage: ./test_all_version_apis.sh
# API_URL=http://localhost:5001 ./test_all_version_apis.sh
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service ./test_all_version_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 Version API Endpoints ==="
echo "API_URL: ${API_URL}"
echo ""

echo "1. Testing GET /ops/version (public endpoint)"
echo "   Command: ${SCRIPT_DIR}/get_version.sh"
${SCRIPT_DIR}/get_version.sh
echo ""

echo "=== V4 Version API Testing Complete ==="