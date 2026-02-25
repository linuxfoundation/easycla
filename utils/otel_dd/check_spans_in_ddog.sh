#!/bin/bash
# Example:
# ./utils/otel_dd/check_spans_in_ddog.sh --skip-e2e | jq -r '.data[].attributes.custom.http.route' | sort | uniq

SKIP_E2E=0
for arg in "$@"; do
  case "$arg" in
    --skip-e2e) SKIP_E2E=1 ;;
    --no-skip-e2e) SKIP_E2E=0 ;;
    *) ;;
  esac
done

payload='{
  "data": {
    "type": "search_request",
    "attributes": {
      "filter": {
        "from": "now-60m",
        "to": "now",
        "query": "service:easycla-backend env:dev"
      },
      "sort": "timestamp",
      "page": { "limit": 5000 }
    }
  }
}'

# jq helper: treat missing as false; accept "true"/true
jq_filter='
def is_e2e:
  (.attributes.custom.easycla.e2e // false)
  | tostring
  | ascii_downcase == "true";

if env.SKIP_E2E == "1" then
  .data |= (map(select(is_e2e | not)))
else
  .
end
'

curl -sS -X POST "https://api.${DD_SITE}/api/v2/spans/events/search" \
  -H "Content-Type: application/json" \
  -H "DD-API-KEY: ${DD_API_KEY}" \
  -H "DD-APPLICATION-KEY: ${DD_APP_KEY}" \
  -d "${payload}" \
| SKIP_E2E="${SKIP_E2E}" jq "${jq_filter}"
