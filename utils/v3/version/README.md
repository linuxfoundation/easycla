# V3 Version API Curl Scripts

This directory contains curl scripts for testing V3 Version API endpoints.

## Available Scripts

All version endpoints are **public** and require **no authentication**.

### `get_version.sh`
Get application version and build information.

```bash
# Local API
API_URL="http://localhost:5001" ./get_version.sh

# Remote API  
API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./get_version.sh

# With debug
DEBUG=1 ./get_version.sh
```

**Expected Response:**
```json
{
  "version": "v3.x.x",
  "commit": "abc123def456...",
  "buildDate": "2024-01-01T00:00:00Z",
  // ... other build information
}
```

### `test_all_version_apis.sh`
Test all version endpoints.

```bash
# Local API
./test_all_version_apis.sh

# Remote API
API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./test_all_version_apis.sh
```

## Environment Variables

- `API_URL`: API base URL (defaults to `http://localhost:5001`)
- `DEBUG`: Set to `1` to print curl commands and raw output

## Notes

- Version endpoints are public and require no authentication
- Used for deployment verification and debugging
- Returns build-time information including version, commit hash, and build date
- Useful for confirming which version is deployed in different environments
- Should always return 200 status when API is operational