#!/bin/bash

for ghid in 132510 261265 368587 695473 698465 836183 1184214 1299064 1560718 2116997 3212351 4453979 5009646 5094756 5557367 6763318 9434922 10579045 12587791 13224827 13757056 13929923 14296719 18719127 19684366 20407524 26163841 26692080 31728060 32516413 37202250 38041311 44139130 56246024 63438915 66864971 72978371 76980726 79828097 82919057 88570905 91555602 94173498 96516301 116630390 131784352 132999137
do
  dev_user_id=$(aws dynamodb query --profile lfproduct-dev --table-name cla-dev-users --index-name github-id-index --key-condition-expression "user_github_id = :gid" --expression-attribute-values '{":gid":{"N":"'"$ghid"'"}}' --limit 1 --query 'Items[0].user_id.S' --output text)
  if [ ! "${dev_user_id}" = "None" ]
  then
    echo "${ghid} already present in dev with id ${dev_user_id}, skipping"
    continue
  fi
  prod_user_id=$(aws dynamodb query --profile lfproduct-prod --table-name cla-prod-users --index-name github-id-index --key-condition-expression "user_github_id = :gid" --expression-attribute-values '{":gid":{"N":"'"$ghid"'"}}' --limit 1 --query 'Items[0].user_id.S' --output text)
  if [ "${prod_user_id}" = "None" ]
  then
    echo "${ghid} not found on prod, skipping"
    continue
  fi
  echo "${ghid} has id ${prod_user_id} on prod, copying to dev"
  ./utils/copy_prod_to_dev.sh users user_id "${prod_user_id}"
  dev_user_id=$(aws dynamodb query --profile lfproduct-dev --table-name cla-dev-users --index-name github-id-index --key-condition-expression "user_github_id = :gid" --expression-attribute-values '{":gid":{"N":"'"$ghid"'"}}' --limit 1 --query 'Items[0].user_id.S' --output text)
  if [ "${dev_user_id}" = "None" ]
  then
    echo "failed to copy ${ghid} (prod id: ${prod_user_id}) to dev"
  else
    echo "copied ${ghid} (prod id: ${prod_user_id}) to dev: ${dev_user_id}"
  fi
done
