#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# Calls the CLA Group search API (GET /v4/cla-group/search) through lfx-gateway and reports the HTTP status and total time.
# SEARCH_TERM (or 1st arg): the term to search for - a CLA Group name, project/foundation name, organization name, or a pasted repository URL or "owner/repo" path (min 3 characters).
# LIMIT: the result cap (1-100, default 20 server-side).
# TOKEN: bearer access token (env, or ./cla_search.token.secret / ./my_clas.token.secret / ./auth0.token.secret). Get one with ~/get_oauth_token.sh (dev) or ~/get_oauth_token_prod.sh (prod).
# STAGE: dev (default) | test | staging | prod - selects the api-gw host.
# Local mode (against a standalone backend, bypassing the gateway): set PRINCIPAL to the token username (or pass a raw base64 X_ACL). Defaults API_URL to http://localhost:8080.
# RUNS: when >1, repeats the call that many times and reports the min/p50/p95/max server time (FR-001a's < 300 ms p95 budget); the body is printed only for the first run.
# Examples:
#   ./utils/cla_search.sh kubernetes
#   SEARCH_TERM="https://github.com/OpenTimelineIO/OpenTimelineIO-Java-Bindings" ./utils/cla_search.sh
#   STAGE=prod TOKEN="$(~/get_oauth_token_prod.sh)" RUNS=30 ./utils/cla_search.sh onap
#   PRINCIPAL=lgryglicki ./utils/cla_search.sh kube      # local standalone backend

if [ -n "$PRINCIPAL" ] && [ -z "$X_ACL" ]
then
  X_ACL="$(printf '{"user_name":"%s","email":"%s","isAdmin":false,"allowed":true}' "$PRINCIPAL" "${PRINCIPAL_EMAIL:-$PRINCIPAL}" | base64 | tr -d '\n')"
fi

if [ -n "$X_ACL" ]
then
  auth=(-H "X-ACL: ${X_ACL}")
  [ -z "$API_URL" ] && API_URL="http://localhost:${PORT:-8080}"
else
  for f in ./cla_search.token.secret ./my_clas.token.secret ./auth0.token.secret
  do
    [ -n "$TOKEN" ] && break
    [ -f "$f" ] && TOKEN="$(cat "$f")"
  done
  if [ -z "$TOKEN" ]
  then
    echo "$0: TOKEN not set - run ~/get_oauth_token.sh (dev) or ~/get_oauth_token_prod.sh (prod) and export TOKEN (or use PRINCIPAL=... for local mode)"
    exit 1
  fi
  auth=(-H "Authorization: Bearer ${TOKEN}")
  [ -z "$STAGE" ] && STAGE=dev
  case "$STAGE" in
    prod) GW="https://api-gw.platform.linuxfoundation.org" ;;
    staging) GW="https://api-gw.staging.platform.linuxfoundation.org" ;;
    test) GW="https://api-gw.test.platform.linuxfoundation.org" ;;
    dev) GW="https://api-gw.dev.platform.linuxfoundation.org" ;;
    *) echo "$0: unknown STAGE '$STAGE'"; exit 2 ;;
  esac
  [ -z "$API_URL" ] && API_URL="${GW}/cla-service"
fi

[ -z "$SEARCH_TERM" ] && SEARCH_TERM="$1"
if [ -z "$SEARCH_TERM" ]
then
  echo "$0: SEARCH_TERM not set - pass it as the first argument or in the environment"
  exit 3
fi

URL="${API_URL}/v4/cla-group/search"
args=(--data-urlencode "searchTerm=${SEARCH_TERM}")
[ -n "$LIMIT" ] && args+=(--data-urlencode "limit=${LIMIT}")

if [ -n "$DEBUG" ]
then
  echo "curl -sS -G -XGET ${auth[0]} '<redacted>' -H 'Content-Type: application/json' ${args[*]} '${URL}'"
fi

[ -z "$RUNS" ] && RUNS=1
if ! printf '%s' "$RUNS" | grep -Eq '^[1-9][0-9]*$'
then
  echo "$0: RUNS must be a positive integer, got '${RUNS}'"
  exit 4
fi

body="$(mktemp)"
times="$(mktemp)"
trap 'rm -f "$body" "$times"' EXIT INT TERM
failed=0
for i in $(seq 1 "$RUNS")
do
  timing="$(curl -sS -G -XGET "${auth[@]}" -H "Content-Type: application/json" "${args[@]}" -w '%{http_code} %{time_total}' -o "$body" "$URL")"
  code="${timing% *}"
  secs="${timing#* }"
  echo "$secs" >> "$times"
  case "$code" in 2??) ;; *) failed=$((failed+1)) ;; esac
  if [ "$i" = "1" ]
  then
    if command -v jq >/dev/null 2>&1
    then
      jq -r '.' < "$body" 2>/dev/null || cat "$body"
    else
      cat "$body"
    fi
    echo
  fi
  [ "$RUNS" = "1" ] && echo "HTTP ${code} in ${secs}s"
done

if [ "$RUNS" != "1" ]
then
  sort -n "$times" | awk -v runs="$RUNS" -v failed="$failed" '{t[NR]=$1} END {
    i50=int((NR+1)*0.50+0.5); if (i50>NR) i50=NR; if (i50<1) i50=1
    i95=int((NR+1)*0.95+0.5); if (i95>NR) i95=NR; if (i95<1) i95=1
    printf "runs=%d min=%.3fs p50=%.3fs p95=%.3fs max=%.3fs", runs, t[1], t[i50], t[i95], t[NR]
    if (failed+0 > 0) printf " NON-2XX=%d (timings above are not a valid measurement)", failed
    printf "\n"
  }'

  if [ "$failed" != "0" ]
  then
    exit 5
  fi
fi
