#!/bin/bash
# API_URL=https://[xyz].ngrok-free.app (defaults to localhost:5000)
if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi
curl -s --fail -XGET "${API_URL}/v2/health"
r=$?
if [[ ${r} -eq 0 ]]
then
  echo "Successful response"
else
  echo "Failed to get a successful response"
  exit ${r}
fi
