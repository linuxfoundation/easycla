#!/bin/bash
# user_id='9dcf5bbc-2492-11ed-97c7-3e2a23ea20b5'
# select key, data:expire from fivetran_ingest.dynamodb_product_us_east1_dev.cla_dev_store where key like 'active_signature:%' order by data:expire desc limit 1;
# select key, data:expire from fivetran_ingest.dynamodb_product_us_east_1.cla_prod_store where key like 'active_signature:%' order by data:expire desc limit 1;
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org DEBUG=1 ./utils/active_signature_py.sh '4b344ac4-f8d9-11ed-ac9b-b29c4ace74e9'
# API_URL=https://api.lfcla.dev.platform.linuxfoundation.org DEBUG=1 ./utils/active_signature_py.sh '4b344ac4-f8d9-11ed-ac9b-b29c4ace74e9'
# API_URL=https://api.easycla.lfx.linuxfoundation.org DEBUG='' ./utils/active_signature_py.sh '564e571e-12d7-4857-abd4-898939accdd7'
# ./utils/add_store_active_signature.sh b817eb57-045a-4fe0-8473-fbb416a01d70 d8cead54-92b7-48c5-a2c8-b1e295e8f7f1
# DEBUG='' ./utils/active_signature_py.sh b817eb57-045a-4fe0-8473-fbb416a01d70 | jq '.'

if [ -z "$1" ]
then
  echo "$0: you need to specify user_id as a 1st parameter"
  exit 1
fi
export user_id="$1"

if [ -z "$API_URL" ]
then
  export API_URL="http://localhost:5000"
fi

if [ ! -z "$DEBUG" ]
then
  echo "curl -s -XGET -H 'Content-Type: application/json' \"${API_URL}/v2/user/${user_id}/active-signature\""
  curl -s -XGET -H "Content-Type: application/json" "${API_URL}/v2/user/${user_id}/active-signature"
else
  curl -s -XGET -H "Content-Type: application/json" "${API_URL}/v2/user/${user_id}/active-signature" | jq -r '.'
fi
