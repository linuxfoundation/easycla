#!/bin/bash
# API_URL=https://3f13-147-75-85-27.ngrok-free.app (defaults to localhost:5000)
# user_id='9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5'
# company_id='862ff296-6508-4f10-9147-2bc2dd7bfe80'
# project_id='88ee12de-122b-4c46-9046-19422054ed8d'
# return_url_type='github'
# return_url='http://localhost'
# DEBUG=1 ./scripts/request_employee_signature_post.sh 9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5 862ff296-6508-4f10-9147-2bc2dd7bfe80 88ee12de-122b-4c46-9046-19422054ed8d github 'http://localhost'

if [ -z "$1" ]
then
  echo "$0: you need to specify user_id as a 1st parameter"
  exit 1
fi
export user_id="$1"

if [ -z "$2" ]
then
  echo "$0: you need to specify company_id as a 2nd parameter"
  exit 2
fi
export company_id="$2"

if [ -z "$3" ]
then
  echo "$0: you need to specify project_id as a 3rd parameter"
  exit 3
fi
export project_id="$3"

if [ -z "$4" ]
then
  echo "$0: you need to specify return_url_type as a 4th parameter: github|gitlab|gerrit"
  exit 4
fi
export return_url_type="$4"

if [ -z "$5" ]
then
  echo "$0: you need to specify return_url as a 5th parameter"
  exit 5
fi
export return_url="$5"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XPOST -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' '${API_URL}/v2/request-employee-signature' -d '{\"project_id\":\"${project_id}\",\"user_id\":\"${user_id}\",\"company_id\":\"${company_id}\",\"return_url_type\":\"${return_url_type}\",\"return_url\":\"${return_url}\"}' | jq -r '.'"
fi
curl -s -XPOST -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v2/request-employee-signature" -d "{\"project_id\":\"${project_id}\",\"user_id\":\"${user_id}\",\"company_id\":\"${company_id}\",\"return_url_type\":\"${return_url_type}\",\"return_url\":\"${return_url}\"}" | jq -r '.'
