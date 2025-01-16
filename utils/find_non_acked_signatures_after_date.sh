#!/bin/bash
# STAGE=dev|prod
# AFTER_DATE='YYYY:MM-DDTHH:MI:SS'
# FROM_FILE=signatures-embargo-not-acked.dev.secret
# DBG=1
# UPDATE=1
# example update: STAGE=prod DBG=1 UPDATE=1 ./utils/find_non_acked_signatures_after_date.sh > find_non_acked_signatures_after_date.prod.log.secret
if [ -z "$STAGE" ]
then
  export STAGE=dev
fi
if [ -z "$AFTER_DATE" ]
then
  export AFTER_DATE='2024-12-17T11:03'
fi
if [ ! -z "$FROM_FILE" ]
then
  signatures=$(cat "${FROM_FILE}")
else
  signatures=$(aws --profile "lfproduct-$STAGE" dynamodb scan --table-name "cla-${STAGE}-signatures" --projection-expression 'signature_id' --filter-expression "(date_created >= :date OR date_modified >= :date) AND (signature_embargo_acked <> :true OR signature_embargo_acked = :null OR attribute_not_exists(signature_embargo_acked))" --expression-attribute-values "{\":date\":{\"S\":\"${AFTER_DATE}\"},\":true\":{\"BOOL\":true},\":null\":{\"NULL\":true}}" | jq -r '.Items[].signature_id.S' | sort | uniq)
fi
for signature in $signatures
do
  echo "signature: $signature"
  if ( [ ! -z "$DBG" ] || [ -z "$UPDATE" ] )
  then
    echo aws --profile "lfproduct-$STAGE" dynamodb update-item --table-name "cla-${STAGE}-signatures" --key "{\"signature_id\":{\"S\":\"${signature}\"}}" --update-expression '"SET signature_embargo_acked = :true"' --expression-attribute-values "{\":true\":{\"BOOL\":true}}"
  fi
  if [ ! -z "$UPDATE" ]
  then
    aws --profile "lfproduct-$STAGE" dynamodb update-item --table-name "cla-${STAGE}-signatures" --key "{\"signature_id\":{\"S\":\"${signature}\"}}" --update-expression "SET signature_embargo_acked = :true" --expression-attribute-values "{\":true\":{\"BOOL\":true}}"
  fi
done

