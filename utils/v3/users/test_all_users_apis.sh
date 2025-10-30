#!/bin/bash
# Test all V3 Users API endpoints using curl scripts
# Usage: ./test_all_users_apis.sh
# API_URL=http://localhost:5001 TOKEN="$(cat ./token.secret)" ./test_all_users_apis.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org TOKEN="$(cat ./token.secret)" ./test_all_users_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if [ -z "$TOKEN" ]
then
  TOKEN="$(cat ./token.secret)"
fi

if [ -z "$TOKEN" ]
then
  echo "$0: TOKEN not specified and unable to obtain one"
  exit 1
fi

if [ -z "$XACL" ]
then
  XACL="$(cat ./x-acl.secret)"
fi

if [ -z "$XACL" ]
then
  echo "$0: XACL not specified and unable to obtain one"
  exit 2
fi

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

export TOKEN
export XACL
export API_URL

echo "=== Testing V3 Users API Endpoints ==="
echo "API_URL: ${API_URL}"
echo "TOKEN: ${TOKEN:0:20}..."
echo "XACL: ${XACL:0:20}..."
echo ""

# Test 1: GET /user-compat/{userID} (public endpoint)
echo "1. Testing GET /user-compat/{userID} (public endpoint)"
echo "   Command: ${SCRIPT_DIR}/get_user_compat.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5"
${SCRIPT_DIR}/get_user_compat.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5
echo ""

# Test 2: GET /users/search (authenticated)
echo "2. Testing GET /users/search (authenticated)"
echo "   Command: ${SCRIPT_DIR}/search_users.sh lukasz true 5"
${SCRIPT_DIR}/search_users.sh lukasz true 5
echo ""

# Test 3: GET /users/{userID} (authenticated)
echo "3. Testing GET /users/{userID} (authenticated)"
echo "   Command: ${SCRIPT_DIR}/get_user.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5"
${SCRIPT_DIR}/get_user.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5
echo ""

# Test 4: GET /users/username/{userName} (authenticated)
echo "4. Testing GET /users/username/{userName} (authenticated)"
echo "   Command: ${SCRIPT_DIR}/get_user_by_username.sh lukaszgryglicki"
${SCRIPT_DIR}/get_user_by_username.sh lukaszgryglicki
echo ""

# Test 5: POST /users (authenticated) - Create user
echo "5. Testing POST /users (authenticated) - Create user"
UNIQUE_ID="$(date +%s)$(shuf -i 1000-9999 -n 1)"
echo "   Command: ${SCRIPT_DIR}/create_user.sh \"TEST${UNIQUE_ID}\" \"testuser${UNIQUE_ID}\" \"testuser${UNIQUE_ID}@example.com\" \"testuser${UNIQUE_ID}\" \"${UNIQUE_ID}\" \"testuser${UNIQUE_ID}gh\" false \"Test user via script\""
CREATED_USER=$(${SCRIPT_DIR}/create_user.sh "TEST${UNIQUE_ID}" "testuser${UNIQUE_ID}" "testuser${UNIQUE_ID}@example.com" "testuser${UNIQUE_ID}" "${UNIQUE_ID}" "testuser${UNIQUE_ID}gh" false "Test user via script")
echo "$CREATED_USER"
CREATED_USER_ID=$(echo "$CREATED_USER" | jq -r '.userID // empty')
echo ""

# Test 6: PUT /users (authenticated) - Update user (only if user was created)
if [ ! -z "$CREATED_USER_ID" ] && [ "$CREATED_USER_ID" != "null" ]; then
  echo "6. Testing PUT /users (authenticated) - Update user"
  echo "   Command: ${SCRIPT_DIR}/update_user.sh \"${CREATED_USER_ID}\" \"Updated via test script\" \"updated${UNIQUE_ID}@example.com\""
  ${SCRIPT_DIR}/update_user.sh "${CREATED_USER_ID}" "Updated via test script" "updated${UNIQUE_ID}@example.com"
  echo ""
  
  # Test 7: DELETE /users/{userID} (authenticated) - Delete user
  echo "7. Testing DELETE /users/{userID} (authenticated) - Delete user"
  echo "   Command: ${SCRIPT_DIR}/delete_user.sh \"${CREATED_USER_ID}\""
  ${SCRIPT_DIR}/delete_user.sh "${CREATED_USER_ID}"
  echo ""
else
  echo "6. Skipping PUT /users - no user was created"
  echo "7. Skipping DELETE /users - no user to delete"
  echo ""
fi

# Test 8: PUT /users (authenticated) - Update non-existent user (should fail)
echo "8. Testing PUT /users (authenticated) - Update non-existent user (expected to fail)"
echo "   Command: ${SCRIPT_DIR}/update_user.sh \"d9428888-122b-4b20-8c4a-0c9a1a6f9b8e\" \"This should fail\" \"nonexistent@example.com\""
${SCRIPT_DIR}/update_user.sh "d9428888-122b-4b20-8c4a-0c9a1a6f9b8e" "This should fail" "nonexistent@example.com"
echo ""

# Test 9: DELETE /users/{userID} (authenticated) - Delete non-existent user (should fail)
echo "9. Testing DELETE /users/{userID} (authenticated) - Delete non-existent user (expected to fail)"
echo "   Command: ${SCRIPT_DIR}/delete_user.sh \"d9428888-122b-4b20-8c4a-0c9a1a6f9b8e\""
${SCRIPT_DIR}/delete_user.sh "d9428888-122b-4b20-8c9a-0c9a1a6f9b8e"
echo ""

echo "=== V3 Users API Testing Complete ==="