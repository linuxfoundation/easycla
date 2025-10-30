#!/bin/bash
# Get API Documentation (public endpoint, no auth required)
# Usage: ./get_api_docs.sh
# API_URL=http://localhost:5001 ./get_api_docs.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./get_api_docs.sh

# Handle API URL
. ./utils/shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v3/api-docs"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\""
USE_JQ=false
. ./utils/shared/handle_curl_execution.sh