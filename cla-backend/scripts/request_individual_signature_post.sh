#!/bin/bash
# API_URL=https://3f13-147-75-85-27.ngrok-free.app (defaults to localhost:5000)
# user_id='9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5'
# project_id='88ee12de-122b-4c46-9046-19422054ed8d'
# return_url_type='github'
# return_url='http://localhost'

if [ -z "$1" ]
then
  echo "$0: you need to specify user_id as a 1st parameter"
  exit 1
fi
export user_id="$1"

if [ -z "$2" ]
then
  echo "$0: you need to specify project_id as a 2nd parameter"
  exit 2
fi
export project_id="$2"

if [ -z "$3" ]
then
  echo "$0: you need to specify return_url_type as a 3rd parameter: github|gitlab|gerrit"
  exit 3
fi
export return_url_type="$3"

if [ -z "$4" ]
then
  echo "$0: you need to specify return_url as a 4th parameter"
  exit 4
fi
export return_url="$4"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XPOST -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' '${API_URL}/v2/request-individual-signature' -d '{\"project_id\":\"${project_id}\",\"user_id\":\"${user_id}\",\"return_url_type\":\"${return_url_type}\",\"return_url\":\"${return_url}\"}' | jq -r '.'"
fi
curl -s -XPOST -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v2/request-individual-signature" -d "{\"project_id\":\"${project_id}\",\"user_id\":\"${user_id}\",\"return_url_type\":\"${return_url_type}\",\"return_url\":\"${return_url}\"}" | jq -r '.'
