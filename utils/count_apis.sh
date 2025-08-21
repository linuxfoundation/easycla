#!/bin/bash
if [ -z "${1}" ]
then
  echo "Usage: $0 <path-to-api-logs>"
  echo "Example: $0 api-logs-prod.json"
  exit 1
fi

jq -r '
  .[].message
  | capture("LG:api-request-path:(?<p>[^\"[:space:]]+)")?  # find the path
  | select(.)                                             # drop non-matches
  | .p
' "${1}" \
| sed -E 's/[0-9a-fA-F-]{36}/<uuid>/g' \
| sed -E ':a;s#/([0-9]{1,})(/|$)#/<id>\2#g;ta' \
| sort | uniq -c | sort -nr
