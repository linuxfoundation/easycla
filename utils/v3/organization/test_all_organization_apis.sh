#!/bin/bash
# Test all V3 Organization API endpoints using curl scripts
# Usage: ./test_all_organization_apis.sh
# API_URL=http://localhost:5001 ./test_all_organization_apis.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./test_all_organization_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

export API_URL

echo "=== Testing V3 Organization API Endpoints ==="
echo "API_URL: ${API_URL}"
echo ""

# Test 1: Search by company name
echo "1. Testing GET /organization/search?companyName=... (public endpoint)"
echo "   Command: ${SCRIPT_DIR}/search_organization.sh \"Linux Foundation\""
${SCRIPT_DIR}/search_organization.sh "Linux Foundation"
echo ""

# Test 2: Search by website name
echo "2. Testing GET /organization/search?websiteName=... (public endpoint)"
echo "   Command: ${SCRIPT_DIR}/search_organization.sh \"\" \"linuxfoundation.org\""
${SCRIPT_DIR}/search_organization.sh "" "linuxfoundation.org"
echo ""

# Test 3: Search by both company name and website
echo "3. Testing GET /organization/search?companyName=...&websiteName=... (public endpoint)"
echo "   Command: ${SCRIPT_DIR}/search_organization.sh \"Linux Foundation\" \"linuxfoundation.org\""
${SCRIPT_DIR}/search_organization.sh "Linux Foundation" "linuxfoundation.org"
echo ""

# Test 4: Search for non-existing organization
echo "4. Testing GET /organization/search with non-existing company"
echo "   Command: ${SCRIPT_DIR}/search_organization.sh \"Non-existing XYZ\""
${SCRIPT_DIR}/search_organization.sh "Non-existing XYZ"
echo ""

echo "=== V3 Organization API Testing Complete ==="