#!/bin/bash
> ./signatures-to-update.secret
for signature_id in $(cat ./signatures-to-check.secret)
do
  echo "checking $signature_id"
  ./utils/fix_signature_approved_flag.sh "${signature_id}" >> ./signatures-to-update.secret
done
