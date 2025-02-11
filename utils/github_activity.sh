#!/bin/bash
# API_URL=https://3f13-147-75-85-27.ngrok-free.app (defaults to localhost:5000)
# DEBUG=1 ./utils/github_activity.sh

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ ! -z "$DEBUG" ]
then
  cat new-pr.json.secret
  echo "curl -s -XPOST -H \"Content-Type: application/json\" -H \"X-GITHUB-EVENT: pull_request\" \"${API_URL}/v2/github/activity\" --data-binary '@new-pr.json.secret'"
fi
curl -s -XPOST -H "Content-Type: application/json" -H "X-GITHUB-EVENT: pull_request" "${API_URL}/v2/github/activity" --data-binary '@new-pr.json.secret'
