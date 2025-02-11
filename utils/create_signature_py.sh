#!/bin/bash
# API_URL=https://3f13-147-75-85-27.ngrok-free.app (defaults to localhost:5000)
# auth_user: check_auth (comment out to bypass)
if ( [ ! -z "$DEBUG" ] && [ -z "$project_id" ] )
then
  echo "$0: example:"
  echo "$0: project_id=88ee12de-122b-4c46-9046-19422054ed8d reference_id=9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5 reference_type=user|company type=cla|ccla|ecla signed=true approved=true embargo_acked=true return_url=https://github.com/VeerSecurityOnbordingOrg/repo11/pull/1 sign_url=https://demo.docusign.net/Signing ccla_company_id='\"8530442c-1805-4a8a-bf1d-cfca6ffc7401\"'|null"
  echo "$0: project_id=88ee12de-122b-4c46-9046-19422054ed8d reference_id=9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5 reference_type=user type=cla signed=false approved=false embargo_acked=true return_url=https://google.com sign_url=https://google.com ccla_company_id=null ./utils/create_signature_py.sh"
fi

if [ -z "$TOKEN" ]
then
  # ./m2m-token-dev.secret
  TOKEN="$(cat ./auth0.token.secret)"
fi

if [ -z "$TOKEN" ]
then
  echo "$0: TOKEN not specified and unable to obtain one"
  exit 1
fi

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

data="{\"signature_project_id\":\"${project_id}\",\"signature_reference_id\":\"${reference_id}\",\"signature_reference_type\":\"${reference_type}\",\"signature_type\":\"${type}\",\"signature_signed\":${signed},\"signature_approved\":${approved},\"signature_embargo_acked\":${embargo_acked},\"signature_return_url\":\"${return_url}\",\"signature_sign_url\":\"${sign_url}\",\"signature_user_ccla_company_id\":${ccla_company_id}}"

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XPOST -H \"Authorization: Bearer ${TOKEN}\" -H \"Content-Type: application/json\" \"${API_URL}/v1/signature\" -d \"${data}\""
  curl -s -XPOST -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v1/signature" -d "${data}"
else
  curl -s -XPOST -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v1/signature" -d "${data}" | jq -r '.'
fi
