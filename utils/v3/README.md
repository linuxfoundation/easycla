# V3 API Curl Scripts

This directory contains comprehensive curl scripts for testing all V3 API endpoints as defined in the Cypress test suites.

## Directory Structure

```
v3/
├── docs/           # Documentation API endpoints (public)
├── health/         # Health check API endpoints (public)  
├── organization/   # Organization search API endpoints (public)
├── users/          # User management API endpoints (authenticated)
├── version/        # Version information API endpoints (public)
└── test_all_v3_apis.sh  # Master test script for all APIs
```

## Quick Start

### Test All APIs (Recommended)

```bash
# Test all public APIs (no authentication required)
API_URL="http://localhost:5001" ./test_all_v3_apis.sh

# Test all APIs including authenticated endpoints
TOKEN="$(cat ./token.secret)" API_URL="http://localhost:5001" ./test_all_v3_apis.sh

# Test against remote API
TOKEN="$(cat ./token.secret)" API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./test_all_v3_apis.sh
```

### Test Individual API Groups

```bash
# Documentation APIs
./docs/test_all_docs_apis.sh

# Health APIs  
./health/test_all_health_apis.sh

# Organization APIs
./organization/test_all_organization_apis.sh

# Version APIs
./version/test_all_version_apis.sh

# Users APIs (requires authentication)
TOKEN="$(cat ./token.secret)" ./users/test_all_users_apis.sh
```

## API Endpoint Summary

### Public Endpoints (No Authentication Required)

| API Group | Endpoints | Description |
|-----------|-----------|-------------|
| Documentation | 2 | API docs and Swagger JSON |
| Health | 1 | Application health status |
| Organization | 1 | Organization search |
| Version | 1 | Application version info |
| **Total** | **5** | |

### Authenticated Endpoints (Require TOKEN and XACL)

| API Group | Endpoints | Description |
|-----------|-----------|-------------|
| Users | 7+ | User CRUD operations and search |
| **Total** | **7+** | |

## Environment Variables

### Global Variables
- `API_URL`: API base URL (defaults to `http://localhost:5001`)
- `DEBUG`: Set to `1` to print curl commands before execution

### Authentication Variables (for authenticated endpoints)
- `TOKEN`: Authentication token (reads from `token.secret` if not provided)
- `XACL`: X-ACL header value (reads from `x-acl.secret` if not provided)

## Prerequisites for Authenticated APIs

1. **Token**: Create a `token.secret` file in the project root with your authentication token
2. **XACL**: Create an `x-acl.secret` file in the project root with your X-ACL header value

```bash
# Example setup
echo "your-auth-token-here" > token.secret
echo "your-xacl-value-here" > x-acl.secret
```

## Usage Examples

### Local Development

```bash
# Test all public APIs locally
API_URL="http://localhost:5001" ./test_all_v3_apis.sh

# Test all APIs including authenticated ones locally
TOKEN="$(cat ./token.secret)" API_URL="http://localhost:5001" ./test_all_v3_apis.sh

# Individual API testing with debug
DEBUG=1 API_URL="http://localhost:5001" ./health/get_health.sh
```

### Remote/Production Testing

```bash
# Test against development environment
TOKEN="$(cat ./token.secret)" API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./test_all_v3_apis.sh

# Test specific organization search
API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./organization/search_organization.sh "Linux Foundation"
```

## Script Conventions

All scripts follow consistent patterns:
- Support `DEBUG=1` for verbose output with curl commands
- Use `jq` for JSON formatting when `DEBUG` is not set
- Provide usage examples and parameter descriptions
- Handle missing parameters with helpful error messages
- Read authentication from files when environment variables not set
- Default to localhost:5001 when API_URL not specified

## Integration with Tests

These curl scripts correspond directly to the test cases in:
- `tests/functional/cypress/e2e/v3/docs.cy.ts`
- `tests/functional/cypress/e2e/v3/health.cy.ts`
- `tests/functional/cypress/e2e/v3/organization.cy.ts`
- `tests/functional/cypress/e2e/v3/users.cy.ts`
- `tests/functional/cypress/e2e/v3/version.cy.ts`

## Use Cases

- **Manual API Testing**: Quick verification of API functionality
- **CI/CD Integration**: Automated API testing in pipelines  
- **Load Testing Preparation**: Generate realistic API calls
- **API Debugging**: Isolate and reproduce API issues
- **Documentation**: Live examples for API consumers
- **Monitoring**: Health checks and basic functionality verification