#!/bin/bash

if [ -z "$SIG_ID" ]
then
  echo "Usage: SIG_ID=... $0"
  exit 1
fi

set -euo pipefail

export AWS_PROFILE=lfproduct-dev
export AWS_REGION=us-east-1
export STAGE=dev
export TABLE="cla-${STAGE}-signatures"
export GH_USERNAME="lukaszgryglicki"
export EMAIL1="lgryglicki@cncf.io"
export EMAIL2="lukaszgryglicki@o2.pl"

aws --profile "${AWS_PROFILE}" --region "${AWS_REGION}" dynamodb get-item \
  --table-name "${TABLE}" \
  --key "{\"signature_id\":{\"S\":\"${SIG_ID}\"}}" \
| jq \
  --arg gh "${GH_USERNAME}" \
  --arg e1 "${EMAIL1}" \
  --arg e2 "${EMAIL2}" '
  def lstrings(x): [ (x // [])[].S ];

  {
    ":gh": {
      "L": (
        (
          lstrings(.Item.github_whitelist.L) + [$gh]
        )
        | unique
        | map({S: .})
      )
    },
    ":em": {
      "L": (
        (
          lstrings(.Item.email_whitelist.L) + [$e1, $e2]
        )
        | unique
        | map({S: .})
      )
    }
  }
' > /tmp/easycla-update-values.json

cat /tmp/easycla-update-values.json
aws --profile "${AWS_PROFILE}" --region "${AWS_REGION}" dynamodb update-item \
  --table-name "${TABLE}" \
  --key "{\"signature_id\":{\"S\":\"${SIG_ID}\"}}" \
  --update-expression "SET github_whitelist = :gh, email_whitelist = :em" \
  --expression-attribute-values file:///tmp/easycla-update-values.json \
  --return-values ALL_NEW

aws --profile "${AWS_PROFILE}" --region "${AWS_REGION}" dynamodb get-item \
  --table-name "${TABLE}" \
  --key "{\"signature_id\":{\"S\":\"${SIG_ID}\"}}" \
| jq --arg gh "${GH_USERNAME}" --arg e1 "${EMAIL1}" --arg e2 "${EMAIL2}" '
  {
    github_whitelist: [(.Item.github_whitelist.L // [])[].S],
    email_whitelist: [(.Item.email_whitelist.L // [])[].S],
    github_match: ([ (.Item.github_whitelist.L // [])[].S ] | index($gh) != null),
    email1_match: ([ (.Item.email_whitelist.L // [])[].S ] | index($e1) != null),
    email2_match: ([ (.Item.email_whitelist.L // [])[].S ] | index($e2) != null)
  }'
