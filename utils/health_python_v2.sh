#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi
curl -s "${API_URL}/v2/health" | jq -r '.'
