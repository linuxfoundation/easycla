#!/bin/bash
# Get application health status (public endpoint, no auth required)
# Usage: ./get_health.sh
# API_URL=http://localhost:5001 ./get_health.sh
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org ./get_health.sh

# Handle API URL
. ./utils/shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v3/ops/health"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh