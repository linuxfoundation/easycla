#!/bin/bash
# Post GitHub activity (webhook endpoint, requires special headers)
# Usage: ./post_github_activity.sh <action> <repository_name> <installation_id>
# Example: ./post_github_activity.sh opened MyRepo/test-repo 12345
# X_HUB_SIGNATURE="sha256=..." X_GITHUB_EVENT="pull_request" ./post_github_activity.sh opened MyRepo/test-repo 12345

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]
then
  echo "$0: you need to specify action, repository_name, and installation_id as parameters"
  echo "Usage: $0 <action> <repository_name> <installation_id>"
  echo "Example: $0 opened MyRepo/test-repo 12345"
  echo "Environment variables needed: X_HUB_SIGNATURE, X_GITHUB_EVENT"
  exit 1
fi

export action="$1"
export repository_name="$2"
export installation_id="$3"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Handle API URL
. ${SCRIPT_DIR}/../shared/handle_api_url.sh

# Build JSON payload - minimal GitHub webhook structure
PAYLOAD=$(cat <<EOF
{
  "action": "${action}",
  "repository": {
    "full_name": "${repository_name}",
    "name": "${repository_name##*/}"
  },
  "installation": {
    "id": ${installation_id}
  }
}
EOF
)

# Set up headers
HEADERS="-H \"Content-Type: application/json\""

if [ ! -z "$X_HUB_SIGNATURE" ]; then
  HEADERS="${HEADERS} -H \"X-Hub-Signature-256: ${X_HUB_SIGNATURE}\""
fi

if [ ! -z "$X_GITHUB_EVENT" ]; then
  HEADERS="${HEADERS} -H \"X-GitHub-Event: ${X_GITHUB_EVENT}\""
fi

# Set up curl execution
API="${API_URL}/v4/github/activity"
CURL_CMD="curl -s -XPOST ${HEADERS} -d '${PAYLOAD}'"
USE_JQ=true
. ./utils/shared/handle_curl_execution.sh