#!/usr/bin/env bash
set -euo pipefail

STAGE=${STAGE:-dev}
APPLY="${APPLY:-0}"
PROFILE="lfproduct-${STAGE}"
REGION=us-east-1
TABLE="cla-${STAGE}-users"
APPLY="${APPLY:-0}"

aws dynamodb scan --profile "$PROFILE" --region "$REGION" --table-name "$TABLE" --projection-expression 'user_id, user_emails' --filter-expression 'attribute_exists(user_emails)' --output json > "${STAGE}_user_emails.json"
cat "${STAGE}_user_emails.json" | jq -c '.Items[] | select(.user_emails.SS != null) | select(.user_emails.SS | map(select(. != ascii_downcase)) | length > 0)' \
| while IFS= read -r item; do
    uid=$(jq -r '.user_id.S' <<<"$item")
    newss=$(jq -c '[.user_emails.SS[] | ascii_downcase] | unique' <<<"$item")   # lower + dedupe
    echo "user $uid -> $newss"
    if [ "$APPLY" = "1" ]
    then
      aws dynamodb update-item --profile "$PROFILE" --region "$REGION" --table-name "$TABLE" \
        --key "{\"user_id\":{\"S\":\"$uid\"}}" \
        --update-expression 'SET user_emails = :e' \
        --expression-attribute-values "{\":e\":{\"SS\":$newss}}" && echo "ok"
    fi
done
