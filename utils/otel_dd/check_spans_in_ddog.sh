#!/bin/bash
set -euo pipefail

# Example:
# ./utils/otel_dd/check_spans_in_ddog.sh --env prod --skip-e2e \
#   | jq -r '.data[].attributes.custom.http.route' | sort | uniq

usage() {
  cat <<'EOF' >&2
Usage:
  check_spans_in_ddog.sh [--env <dev|prod|...>] [--stage <dev|prod|...>] [--skip-e2e|--no-skip-e2e]

Env vars:
  DD_SITE, DD_API_KEY, DD_APP_KEY  (required)
  DD_ENV / ENV / STAGE             (optional; default "dev")
  DD_SERVICE                       (optional; default "easycla-backend")

Notes:
  - Filters Datadog span events by: service:<DD_SERVICE> env:<DD_ENV>
  - Skips spans where attributes.custom.easycla.e2e == "true" when --skip-e2e
EOF
}

SKIP_E2E=0
DD_ENV="${DD_ENV:-${ENV:-${STAGE:-dev}}}"
DD_SERVICE="${DD_SERVICE:-easycla-backend}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-e2e) SKIP_E2E=1; shift ;;
    --no-skip-e2e) SKIP_E2E=0; shift ;;
    --env|--environment|--stage)
      [[ $# -ge 2 ]] || { echo "ERROR: $1 requires a value" >&2; usage; exit 2; }
      DD_ENV="$2"
      shift 2
      ;;
    --service)
      [[ $# -ge 2 ]] || { echo "ERROR: --service requires a value" >&2; usage; exit 2; }
      DD_SERVICE="$2"
      shift 2
      ;;
    -h|--help) usage; exit 0 ;;
    *)
      echo "ERROR: unknown arg: $1" >&2
      usage
      exit 2
      ;;
  esac
done

QUERY="service:${DD_SERVICE} env:${DD_ENV}"

# Build request JSON safely with jq (avoids quoting bugs)
payload="$(jq -n --arg query "$QUERY" '{
  data: {
    type: "search_request",
    attributes: {
      filter: { from: "now-60m", to: "now", query: $query },
      sort: "timestamp",
      page: { limit: 5000 }
    }
  }
}')"

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
