# V3 Organization API Curl Scripts

This directory contains curl scripts for testing V3 Organization API endpoints.

## Available Scripts

All organization endpoints are **public** and require **no authentication**.

### `search_organization.sh`
Search organizations by company name and/or website name.

```bash
# Search by company name only
./search_organization.sh "Linux Foundation"

# Search by website name only  
./search_organization.sh "" "linuxfoundation.org"

# Search by both company name and website
./search_organization.sh "Linux Foundation" "linuxfoundation.org"

# Remote API
API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./search_organization.sh "Linux Foundation"

# With debug
DEBUG=1 ./search_organization.sh "Linux Foundation"
```

**Parameters:**
1. `companyName` - Company name to search for (optional if websiteName provided)
2. `websiteName` - Website name to search for (optional if companyName provided)

**Expected Response:**
```json
{
  "list": [
    {
      "id": "...",
      "name": "Linux Foundation",
      "website": "https://linuxfoundation.org",
      // ... other organization fields
    }
  ]
}
```

### `test_all_organization_apis.sh`
Test all organization endpoints with various search parameters.

```bash
# Local API
./test_all_organization_apis.sh

# Remote API
API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./test_all_organization_apis.sh
```

This script tests:
1. Search by company name
2. Search by website name  
3. Search by both parameters
4. Search for non-existing organization (should return empty list)

## Environment Variables

- `API_URL`: API base URL (defaults to `http://localhost:5001`)
- `DEBUG`: Set to `1` to print curl commands and raw output

## Notes

- Organization endpoints are public and require no authentication
- At least one search parameter (companyName or websiteName) is required
- Website names must be valid URLs when provided
- Search returns an array of matching organizations
- Empty results return `{"list": []}` with 200 status
- Case-insensitive partial matching is typically supported