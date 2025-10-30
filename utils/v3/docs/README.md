# V3 Documentation API Curl Scripts

This directory contains curl scripts for testing V3 Documentation API endpoints.

## Available Scripts

All documentation endpoints are **public** and require **no authentication**.

### `get_api_docs.sh`
Get API documentation in HTML format.

```bash
# Local API
API_URL="http://localhost:5001" ./get_api_docs.sh

# Remote API  
API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./get_api_docs.sh

# With debug
DEBUG=1 ./get_api_docs.sh
```

### `get_swagger_json.sh`
Get the complete Swagger/OpenAPI JSON specification.

```bash
# Local API
API_URL="http://localhost:5001" ./get_swagger_json.sh

# Remote API
API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./get_swagger_json.sh

# With debug (raw output)
DEBUG=1 ./get_swagger_json.sh
```

### `test_all_docs_apis.sh`
Test all documentation endpoints.

```bash
# Local API
./test_all_docs_apis.sh

# Remote API
API_URL="https://api.lfcla.dev.platform.linuxfoundation.org" ./test_all_docs_apis.sh
```

## Environment Variables

- `API_URL`: API base URL (defaults to `http://localhost:5001`)
- `DEBUG`: Set to `1` to print curl commands and raw output

## Notes

- Documentation endpoints are public and require no authentication
- The API documentation is served as HTML content
- The Swagger JSON contains the complete API specification
- No rate limiting is typically applied to documentation endpoints