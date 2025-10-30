#!/bin/bash
# Shared handler for authentication (TOKEN + XACL)
# Sources both handle_token.sh and handle_xacl.sh
# Usage: . ./utils/shared/handle_auth.sh

# Get the directory where this script is located
SHARED_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source token and XACL handlers
. "$SHARED_DIR/handle_token.sh"
. "$SHARED_DIR/handle_xacl.sh"