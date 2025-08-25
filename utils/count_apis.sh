#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

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
| sed -E 's#/(00|a0)[A-Za-z0-9]{13,16}(/|$)#/<sfid>\2#g' \
| sed -E 's#/lf[A-Za-z0-9]{16,22}(/|$)#/<lfxid>\1#g' \
| sed -E 's#/null(/|$)#/<null>\1#g' \
| sort | uniq -c | sort -nr
