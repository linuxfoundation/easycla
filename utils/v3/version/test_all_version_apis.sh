#!/bin/bash
# Test all V3 Version API endpoints using curl scripts
# Usage: ./test_all_version_apis.sh
# API_URL=http://localhost:5001 ./test_all_version_apis.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./test_all_version_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

export API_URL

echo "=== Testing V3 Version API Endpoints ==="
echo "API_URL: ${API_URL}"
echo ""

# Test 1: GET /ops/version (public endpoint)
echo "1. Testing GET /ops/version (public endpoint)"
echo "   Command: ${SCRIPT_DIR}/get_version.sh"
${SCRIPT_DIR}/get_version.sh
echo ""

echo "=== V3 Version API Testing Complete ==="