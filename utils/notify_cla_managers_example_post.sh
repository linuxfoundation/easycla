#!/bin/bash
# API_URL=ttps://api-gw.platform.linuxfoundation.org/cla-service
# API_URL=ttps://api-gw.dev.platform.linuxfoundation.org/cla-service
# STAGE=prod ./utils/scan.sh projects project_id d8cead54-92b7-48c5-a2c8-b1e295e8f7f1
# STAGE=prod ./utils/scan.sh projects-cla-groups project_name 'Cloud Native Computing Foundation (CNCF)'
# STAGE=prod ./utils/scan.sh companies company_name 'Red Hat, Inc.'
# STAGE=prod ./utils/scan.sh users lf_username lgryglicki
# API_URL=https://api-gw.platform.linuxfoundation.org/cla-service ./utils/notify_cla_managers_example_post.sh

if [ -z "$TOKEN" ]
then
  # source ./auth0_token.secret
  TOKEN="$(cat ./auth0.token.secret)"
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

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5001"
fi

data='{
  "companyName": "Cloud Native Computing Foundation (CNCF)",
  "claGroupID": "d8cead54-92b7-48c5-a2c8-b1e295e8f7f1",
  "userID": "2c895887-d33a-11ef-9205-4e2baeedbda2",
  "list": [
    { "email": "lukaszgryglicki@o2.pl", "name": "Lukasz Gryglicki 1" },
    { "email": "lgryglicki@cncf.io", "name": "Lukasz Gryglicki 2" },
    { "email": "lgryglicki@contractor.linuxfoundation.org", "name": "Lukasz Gryglicki 3" }
  ]
}'

curl -s -XPOST -H "X-ACL: ${XACL}" -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "${API_URL}/v4/notify-cla-managers" -d "$data" | jq -r '.'
