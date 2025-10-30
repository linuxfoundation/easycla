# V3 Health API Curl Scripts

This directory contains curl scripts for testing V3 Health API endpoints.

## Available Scripts

All health endpoints are **public** and require **no authentication**.

### `get_health.sh`
Get application health status.

```bash
# Local API
API_URL="http://localhost:5001" ./get_health.sh

# Remote API  
API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./get_health.sh

# With debug
DEBUG=1 ./get_health.sh
```

**Expected Response:**
```json
{
  "Status": "healthy"
}
```

### `test_all_health_apis.sh`
Test all health endpoints.

```bash
# Local API
./test_all_health_apis.sh

# Remote API
API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./test_all_health_apis.sh
```

## Environment Variables

- `API_URL`: API base URL (defaults to `http://localhost:5001`)
- `DEBUG`: Set to `1` to print curl commands and raw output

## Notes

- Health endpoints are public and require no authentication
- Used for monitoring and load balancer health checks
- Should always return 200 status with `{"Status": "healthy"}` when API is operational
- Typically called frequently by monitoring systems