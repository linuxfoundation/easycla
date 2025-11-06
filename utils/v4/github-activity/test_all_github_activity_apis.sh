#!/bin/bash
# Test ALL V4 GitHub Activity API endpoints
# Usage: ./test_all_github_activity_apis.sh
# X_HUB_SIGNATURE="sha256=..." X_GITHUB_EVENT="pull_request" ./test_all_github_activity_apis.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

echo "=== Testing V4 GitHub Activity API Endpoints ==="
echo "API_URL: ${API_URL}"
echo ""

echo "1. Testing POST /github/activity (webhook endpoint)"
echo "   Command: ${SCRIPT_DIR}/post_github_activity.sh <action> <repo> <installation-id>"
echo "   [Skipping - requires valid webhook signature and GitHub event headers]"
echo "   Note: This endpoint requires X_HUB_SIGNATURE and X_GITHUB_EVENT environment variables"
echo ""

echo "=== V4 GitHub Activity API Testing Complete ==="