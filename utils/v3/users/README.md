# V3 Users API Curl Scripts

This directory contains curl scripts for testing all V3 Users API endpoints as defined in the OpenAPI specification.

## Prerequisites

1. **Token**: Create a `token.secret` file in the project root with your authentication token
2. **XACL**: Create an `x-acl.secret` file in the project root with your X-ACL header value

## Environment Variables

- `TOKEN`: Authentication token (reads from `token.secret` if not provided)
- `XACL`: X-ACL header value (reads from `x-acl.secret` if not provided)  
- `API_URL`: API base URL (defaults to `http://localhost:5001`)
- `DEBUG`: Set to `1` to print curl commands before execution

## Available Scripts

### Public Endpoints (No Authentication Required)

#### `get_user_compat.sh`
Get user information via public compatibility endpoint.

```bash
# Local API
API_URL="http://localhost:5001" ./get_user_compat.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5

# Remote API  
API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./get_user_compat.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5

# With debug
DEBUG=1 ./get_user_compat.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5
```

### Authenticated Endpoints (Require TOKEN and XACL)

#### `search_users.sh`
Search users with optional parameters.

```bash
# Basic search
TOKEN="$(cat ./token.secret)" ./search_users.sh

# Search with parameters
TOKEN="$(cat ./token.secret)" ./search_users.sh "lukasz" true 50

# Remote API
TOKEN="$(cat ./token.secret)" API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./search_users.sh "lukasz"
```

#### `get_user.sh`
Get user by ID (authenticated version).

```bash
TOKEN="$(cat ./token.secret)" ./get_user.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5
```

#### `get_user_by_username.sh`
Get user by username.

```bash
TOKEN="$(cat ./token.secret)" ./get_user_by_username.sh lukaszgryglicki
```

#### `create_user.sh`
Create a new user.

```bash
TOKEN="$(cat ./token.secret)" ./create_user.sh "12345ABC" "testuser123" "testuser123@example.com" "testuser123" "123456" "testuser123gh" false "Test user"
```

Parameters:
1. `userExternalID` - External ID for the user
2. `username` - Username
3. `lfEmail` - Linux Foundation email
4. `lfUsername` - Linux Foundation username
5. `githubID` - GitHub ID
6. `githubUsername` - GitHub username
7. `admin` - Admin flag (optional, defaults to false)
8. `note` - Note about the user (optional)

#### `update_user.sh`
Update an existing user.

```bash
TOKEN="$(cat ./token.secret)" ./update_user.sh "9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5" "Updated note" "email1@example.com,email2@example.com"
```

Parameters:
1. `userID` - ID of user to update
2. `note` - Updated note (optional)
3. `emails` - Comma-separated list of emails (optional)

#### `delete_user.sh`
Delete a user by ID.

```bash
TOKEN="$(cat ./token.secret)" ./delete_user.sh d9428888-122b-4b20-8c4a-0c9a1a6f9b8e
```

## Test All APIs

Use the comprehensive test script to test all endpoints:

```bash
# Local API
TOKEN="$(cat ./token.secret)" ./test_all_users_apis.sh

# Remote API
TOKEN="$(cat ./token.secret)" API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./test_all_users_apis.sh

# With debug output
DEBUG=1 TOKEN="$(cat ./token.secret)" ./test_all_users_apis.sh
```

This script will:
1. Test all GET endpoints
2. Attempt to create a user
3. If creation succeeds, update and delete the user
4. Test error cases with non-existent users

## Expected Behavior

**Note**: The current API implementation restricts user creation, typically returning 409 (Conflict) for POST requests. This is expected behavior for production security. The scripts handle these responses gracefully.

## Script Style

All scripts follow the established pattern from the project's existing utils scripts:
- Default to `localhost:5001` if `API_URL` not provided
- Read `TOKEN` from `token.secret` if not provided as environment variable
- Read `XACL` from `x-acl.secret` if not provided as environment variable
- Support `DEBUG=1` to show curl commands
- Use `jq` for pretty JSON formatting when `DEBUG` is not set
- Proper error handling and usage messages

## Integration with Tests

These curl scripts correspond directly to the test cases in:
- `tests/functional/cypress/e2e/v3/users.cy.ts`

The scripts can be used for:
- Manual API testing
- CI/CD integration
- API debugging and development
- Load testing preparation