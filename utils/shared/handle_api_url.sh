#!/bin/bash
# Shared handler for API_URL environment variable
# Sets API_URL to default localhost if not provided
# Usage: . ./utils/shared/handle_api_url.sh

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi