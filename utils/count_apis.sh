#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

if [ -z "${1}" ]
then
  echo "Usage: $0 <path-to-api-logs> [min-count]"
  echo "Example: $0 api-logs-prod.json"
  exit 1
fi
N=10
if [ ! -z "${2}" ]
then
  N="${2}"
fi
jq -r '
  .[].message
  | capture("LG:api-request-path:(?<p>[^\"[:space:]]+)")?  # find the path
  | select(.)                                             # drop non-matches
  | .p
' "${1}" \
| sed -E 's#/{2,}#/#g' \
| sed -E 's#/$##' \
| sed -E 's#\.(png|svg|css|js|json|xml)$#.<asset>#g' \
| sed -E 's#^/v[0-9]+/swagger\.<asset>$#/v*/swagger#g' \
| sed -E 's#^/v[0-9]+/api-docs$#/v*/api-docs#g' \
| sed -E 's#^/v[0-9]+/(api/)?graphql(\.php)?$#/v*/graphql#g' \
| sed -E 's#^/v[0-9]+/graph(i)?ql(/.*)?$#/v*/graphiql#g' \
| sed -E 's#^/v[0-9]+/(explorer|console|playground|altair)$#/v*/graphql-ui#g' \
| sed -E 's#/([A-Za-z0-9]{5,8})(/|$)#/<shortid>\2#g' \
| sed -E 's/[0-9a-fA-F-]{36}/<uuid>/g' \
| sed -E ':a;s#/([0-9]{1,})(/|$)#/<id>\2#g;ta' \
| sed -E 's#/(00|a0)[A-Za-z0-9]{13,16}(/|$)#/<sfid>\2#g' \
| sed -E 's#/lf[A-Za-z0-9]{16,22}(/|$)#/<lfxid>\1#g' \
| sed -E 's#/null(/|$)#/<null>\1#g' \
| sort | uniq -c | sort -nr \
| awk -v N="$N" '$1 >= N'
