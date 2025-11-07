#!/bin/bash
# Test ALL V4 Health API endpoints
# Usage: ./test_all_health_apis.sh
# API_URL=http://localhost:5001 ./test_all_health_apis.sh
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service ./test_all_health_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 Health API Endpoints ==="
echo "API_URL: ${API_URL}"
echo ""

echo "1. Testing GET /ops/health (public endpoint)"
echo "   Command: ${SCRIPT_DIR}/get_health.sh"
${SCRIPT_DIR}/get_health.sh
echo ""

echo "=== V4 Health API Testing Complete ==="