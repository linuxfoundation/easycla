#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# Returns the LF-wide identities connected to the given LF username in the platform user-service - the same source the My CLAs ownership enforcement uses to authorize keys not present on the EasyCLA records.
# Prints the profile (SFID + profile emails) and the connected identities (Source/Username/Email/DataSource/IsVerified), paginated.
# TOKEN: bearer access token (env, or ./my_clas.token.secret / ./auth0.token.secret). STAGE: dev (default) | test | staging | prod.
# Usage: [STAGE=dev] TOKEN=... ./utils/get_user_svc.sh <lfid>

if [ -z "$1" ]
then
  echo "$0: you need to specify lfid as a 1st parameter, for example: 'lgryglicki'"
  exit 1
fi
lfid="$1"

if [ -z "$TOKEN" ]
then
  [ -f ./my_clas.token.secret ] && TOKEN="$(cat ./my_clas.token.secret)"
fi
if [ -z "$TOKEN" ]
then
  [ -f ./auth0.token.secret ] && TOKEN="$(cat ./auth0.token.secret)"
fi
if [ -z "$TOKEN" ]
then
  echo "$0: TOKEN not set - run ~/get_oauth_token.sh (dev) or ~/get_oauth_token_prod.sh (prod) and export TOKEN"
  exit 1
fi

if [ -z "$STAGE" ]
then
  STAGE=dev
fi
case "$STAGE" in
  prod) GW="https://api-gw.platform.linuxfoundation.org" ;;
  staging) GW="https://api-gw.staging.platform.linuxfoundation.org" ;;
  test) GW="https://api-gw.test.platform.linuxfoundation.org" ;;
  dev) GW="https://api-gw.dev.platform.linuxfoundation.org" ;;
  *) echo "$0: unknown STAGE '$STAGE'"; exit 2 ;;
esac

hdr=(-H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json")
[ -n "$X_API_KEY" ] && hdr+=(-H "X-API-KEY: ${X_API_KEY}")

profile_url="${GW}/user-service/v1/users?username=${lfid}"
if [ -n "$DEBUG" ]
then
  echo "curl -sS -XGET -H 'Authorization: Bearer ...' '${profile_url}'"
fi
profile="$(curl -sS -G -XGET "${hdr[@]}" --data-urlencode "username=${lfid}" "${GW}/user-service/v1/users")"
echo "profile:"
echo "$profile" | jq -r '.Data[0] | {sfid: .ID, name: .Name, emails: [.Emails[]? | {email: .EmailAddress, isPrimary: .IsPrimary, isVerified: .IsVerified, isDeleted: .IsDeleted}]}'

sfid="$(echo "$profile" | jq -r '.Data[0].ID // empty')"
if [ -z "$sfid" ]
then
  echo "no user-service profile found for '${lfid}'"
  exit 0
fi

echo "identities:"
offset=0
while :
do
  if [ -n "$DEBUG" ]
  then
    echo "curl -sS -XGET '${GW}/user-service/v1/users/${sfid}/identities?pageSize=100&offset=${offset}'"
  fi
  page="$(curl -sS -XGET "${hdr[@]}" "${GW}/user-service/v1/users/${sfid}/identities?pageSize=100&offset=${offset}")"
  echo "$page" | jq -r '.Data[]? | {source: .Source, username: .Username, email: .Email, dataSource: .DataSource, isVerified: .IsVerified}'
  count="$(echo "$page" | jq -r '.Data | length')"
  if [ -z "$count" ] || [ "$count" -lt 100 ]
  then
    break
  fi
  offset=$((offset + 100))
done
