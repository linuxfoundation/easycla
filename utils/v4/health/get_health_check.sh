#!/bin/bash
# GET /ops/health
# Healthcheck (public endpoint, no auth required)
# Usage: ./get_health_check.sh
# API_URL=http://localhost:5001 ./get_health_check.sh
# API_URL=https://api-gw.dev.platform.linuxfoundation.org/cla-service ./get_health_check.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Set up curl execution
API="${API_URL}/v4/ops/health"
CURL_CMD="curl -s -XGET -H \"Content-Type: application/json\""
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh
