#!/bin/bash
# Get Swagger JSON specification (public endpoint, no auth required)
# Usage: ./get_swagger_json.sh
# API_URL=http://localhost:5001 ./get_swagger_json.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./get_swagger_json.sh

# Handle API URL
. ./utils/shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v3/swagger.json"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh