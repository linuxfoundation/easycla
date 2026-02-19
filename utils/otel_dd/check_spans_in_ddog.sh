curl -sS -X POST "https://api.${DD_SITE}/api/v2/spans/events/search" \
  -H "Content-Type: application/json" \
  -H "DD-API-KEY: ${DD_API_KEY}" \
  -H "DD-APPLICATION-KEY: ${DD_APP_KEY}" \
  -d '{
    "data": {
      "type": "search_request",
      "attributes": {
        "filter": {
          "from": "now-15m",
          "to": "now",
          "query": "service:easycla-backend env:dev"
        },
        "sort": "timestamp",
        "page": { "limit": 10 }
      }
    }
  }' | jq -r
