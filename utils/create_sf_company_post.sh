#!/bin/bash
# API_URL="https://api-gw.platform.linuxfoundation.org/cla-service"
# API_URL="https://api-gw.dev.platform.linuxfoundation.org/cla-service"
# TOKEN='...' - Auth0 JWT bearer token
# XACL='...' - X-ACL header

if [ -z "$API_URL" ]
then
  API_URL="https://api-gw.dev.platform.linuxfoundation.org/cla-service"
fi

if [ -z "$TOKEN" ]
then
  TOKEN="$(cat ./token.secret)"
fi

if [ -z "$TOKEN" ]
then
  echo "$0: TOKEN not specified and unable to obtain one"
  exit 1
fi

if [ -z "$XACL" ]
then
  XACL="$(cat ./x-acl.secret)"
fi

if [ -z "$XACL" ]
then
  echo "$0: XACL not specified and unable to obtain one"
  exit 2
fi

if [ -z "$1" ]
then
  echo "$0: you need to specify user_id as a 1st parameter"
  exit 3
fi
export user_id="$1"

if [ -z "$2" ]
then
  echo "$0: you need to specify company_name as a 2nd parameter"
  exit 4
fi
export company_name="$2"

if [ -z "$3" ]
then
  echo "$0: you need to specify user_email as a 3rd parameter"
  exit 5
fi
export user_email="$3"

if [ -z "$4" ]
then
  echo "$0: you need to specify company_website as a 4th parameter"
  exit 6
fi
# export company_website=$(jq -rn --arg x "$4" '$x|@uri')
export company_website="$4"

if [ -z "$5" ]
then
  export company_note="Example note."
else
  export company_note="$5"
fi

if [ -z "$6" ]
then
  export company_signing_name="$company_name"
else
  export company_signing_name="$6"
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XPOST -H 'X-ACL: ${XACL}' -H 'Authorization: Bearer ${TOKEN}' -H 'Content-Type: application/json' '${API_URL}/v4/user/${user_id}/company' -d '{\"companyName\":\"${company_name}\",\"companyWebsite\":\"${company_website}\",\"note\":\"${company_note}\",\"signingEntityName\":\"${company_signing_name}\",\"userEmail\":\"${user_email}\"}'"
fi
curl -s -XPOST -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v4/user/${user_id}/company" -d "{\"companyName\":\"${company_name}\",\"companyWebsite\":\"${company_website}\",\"note\":\"${company_note}\",\"signingEntityName\":\"${company_signing_name}\",\"userEmail\":\"${user_email}\"}" | jq -r '.'

