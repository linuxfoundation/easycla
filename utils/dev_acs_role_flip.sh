#!/bin/bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT

# DEV-ONLY helper to flip LF-admin status and manage ACS role grants for a user,
# so DISALLOW_ADMIN EasyCLA v4 ops can be tested with your own token.
#
# Admin flag: the lfx-gateway acs-authorizer plugin asks the ACS warden
# (POST /acs/v1/api/warden/subjects/authorize/v2) which returns isAdmin=true
# when the user holds the 'system-admin' (admin/*) role - 'lf-staff' (staff/*)
# is toggled together with it for consistency. Both are restored losslessly
# (fixed role_id/object_id/object_type_id).
#
# Auth: mints a platform M2M token from the cla-* SSM parameters using the
# lfproduct-dev AWS profile - independent of your personal user token, so you
# can always flip admin back on.
#
# Caching notes:
#  - warden caches successful (user,resource) answers for ~10 minutes; the
#    authoritative current state is the rolescopes listing ('status'/'roles').
#  - after flipping, pass -H 'Cache-Control: no-cache' on your first API call
#    through the gateway to bypass its cached X-ACL for that path.
#
# Usage:
#   ./dev_acs_role_flip.sh status [username]
#   ./dev_acs_role_flip.sh roles [username]                 # full rolescopes JSON
#   ./dev_acs_role_flip.sh admin on|off [username]
#   ./dev_acs_role_flip.sh grant <role> <object_type> <object_id> [username]
#   ./dev_acs_role_flip.sh revoke <role> <object_id> [username]   # object_id '*' allowed
#   ./dev_acs_role_flip.sh warden <resource> [method] [username]  # raw warden probe
#
# Examples:
#   ./dev_acs_role_flip.sh admin off
#   ./dev_acs_role_flip.sh grant cla-manager 'project|organization' 'a09P000000DsCE5IAN|0014100000Te0G7AAJ'
#   ./dev_acs_role_flip.sh revoke cla-manager 'a09P000000DsCE5IAN|0014100000Te0G7AAJ'
#   ./dev_acs_role_flip.sh warden /v4/company/external/0014100000Te0yqAAB/cla-groups GET
#
# Common roles: cla-manager, cla-manager-designee, cla-signatory (object_type
# 'project|organization', object_id '<projectSFID>|<orgSFID>'), system-admin
# ('admin'/'*'), lf-staff ('staff'/'*'). 'roles' shows what you already have.

set -euo pipefail

STAGE="${STAGE:-dev}"
if [ "$STAGE" != "dev" ]; then
  echo "refusing to run: STAGE='$STAGE' - this tool is dev-only" >&2
  exit 1
fi

PROFILE="${AWS_PROFILE_OVERRIDE:-lfproduct-dev}"
REGION="${AWS_REGION_OVERRIDE:-us-east-1}"
ACS="https://api-gw.dev.platform.linuxfoundation.org/acs/v1/api"
DEFAULT_USER="${ACS_USER:-lgryglicki}"
TOKEN_CACHE="/tmp/.lfx_m2m_token_${STAGE}"

ssm() {
  aws ssm get-parameter --profile "$PROFILE" --region "$REGION" \
    --name "cla-auth0-platform-$1-${STAGE}" --query Parameter.Value --output text
}

mint_token() {
  local url cid sec aud resp
  url=$(ssm url); cid=$(ssm client-id); sec=$(ssm client-secret); aud=$(ssm audience)
  resp=$(curl -s --max-time 20 -X POST "$url" -H "Content-Type: application/json" \
    -d "{\"grant_type\":\"client_credentials\",\"client_id\":\"$cid\",\"client_secret\":\"$sec\",\"audience\":\"$aud\"}")
  python3 -c "import json,sys;d=json.loads(sys.argv[1]);tok=d.get('access_token') or sys.exit('token mint failed: '+str(d));print(tok)" "$resp"
}

get_token() {
  if [ -f "$TOKEN_CACHE" ] && [ -n "$(find "$TOKEN_CACHE" -mmin -50 2>/dev/null)" ]; then
    cat "$TOKEN_CACHE"
    return
  fi
  local tok
  tok=$(mint_token)
  (umask 077; echo "$tok" > "$TOKEN_CACHE")
  echo "$tok"
}

acs_get()    { curl -s --max-time 20 -H "Authorization: Bearer $TOKEN" "$ACS$1"; }
acs_post()   { curl -s --max-time 20 -X POST "$ACS$1" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d "$2" -w "\n%{http_code}"; }
acs_delete() { curl -s --max-time 20 -X DELETE "$ACS$1" -H "Authorization: Bearer $TOKEN" -o /dev/null -w "%{http_code}"; }

role_id() {
  case "$1" in
    system-admin) echo cce3551b-12cc-436e-a3c5-682b59afe3b1; return;;
    lf-staff)     echo 0049beb2-2b37-458d-866d-3e42bb1dd6d5; return;;
  esac
  acs_get "/roles?search=$1" | python3 -c "
import json,sys
want=sys.argv[1]
d=json.load(sys.stdin)
rows=d if isinstance(d,list) else d.get('data',[])
for r in rows:
    if r.get('role_name')==want:
        print(r['role_id']); break
else:
    sys.exit('role not found: '+want)" "$1"
}

object_type_id() {
  case "$1" in
    project) echo 1; return;; organization) echo 2; return;; admin) echo 3; return;;
    'project|organization') echo 11; return;; staff) echo 14; return;;
  esac
  acs_get "/object-types" | python3 -c "
import json,sys
want=sys.argv[1]
for t in json.load(sys.stdin):
    if t.get('name')==want:
        print(t['type_id']); break
else:
    sys.exit('object type not found: '+want)" "$1"
}

rolescopes() { acs_get "/users/rolescopes?usernames=$1"; }

grant_ids_for() {  # user role [object_id]
  rolescopes "$1" | python3 -c "
import json,sys
user,role=sys.argv[1],sys.argv[2]
objid=sys.argv[3] if len(sys.argv)>3 else None
d=json.load(sys.stdin)
for r in d.get(user,[]):
    if r['role_name']!=role: continue
    for s in r.get('scopes',[]):
        if objid is None or s.get('object_id')==objid:
            print(r['role_id'],s['grant_id'],s.get('object_type_name',''),s.get('object_id',''))" "$@"
}

do_grant() {  # role objtype objid user
  local rid tid out code
  rid=$(role_id "$1"); tid=$(object_type_id "$2")
  out=$(acs_post "/roles/$rid/members/users" \
    "{\"role_id\":\"$rid\",\"usernames\":[\"$4\"],\"object_ids\":[\"$3\"],\"object_type_id\":$tid}")
  code=${out##*$'\n'}
  case "$code" in
    201) echo "granted: $1 $2/$3 -> $4";;
    409) echo "already granted: $1 $2/$3 -> $4";;
    *) echo "grant $1 $2/$3 -> $4 failed: HTTP $code"; echo "$out" | head -c 400; echo; exit 1;;
  esac
}

do_revoke() {  # role objid user
  local found rid gid rest code
  found=$(grant_ids_for "$3" "$1" "$2")
  if [ -z "$found" ]; then
    echo "no matching grant: $1 $2 for $3"
    return
  fi
  while read -r rid gid rest; do
    code=$(acs_delete "/roles/$rid/members/users/$gid")
    if [ "$code" = "404" ]; then
      echo "revoke $1 grant $gid ($rest): already gone (404 - ACS read lag)"
    else
      echo "revoke $1 grant $gid ($rest): HTTP $code"
    fi
  done <<< "$found"
}

warden_probe() {  # resource method user
  curl -s --max-time 20 -X POST "$ACS/warden/subjects/authorize/v2" \
    -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
    -d "{\"username\":\"$3\",\"resource\":\"$1\",\"action\":\"$2\"}"
}

admin_state() {  # user -> prints true/false from rolescopes
  rolescopes "$1" | python3 -c "
import json,sys
names=[r['role_name'] for r in json.load(sys.stdin).get(sys.argv[1],[])]
print('true' if ('system-admin' in names or 'lf-staff' in names) else 'false')" "$1"
}

wait_admin_state() {  # user expected(true|false)
  local i state
  for i in $(seq 1 10); do
    state=$(admin_state "$1")
    if [ "$state" = "$2" ]; then
      echo "verified: LF admin = $2 for $1 (rolescopes settled)"
      return 0
    fi
    sleep 2
  done
  echo "WARNING: rolescopes still report LF admin = $state (expected $2) after 20s - ACS read lag; re-check with: $0 status $1" >&2
  return 1
}

cmd="${1:-}"; shift || true
case "$cmd" in
  status)
    USER_NAME="${1:-$DEFAULT_USER}"; TOKEN=$(get_token)
    rolescopes "$USER_NAME" | python3 -c "
import json,sys
user=sys.argv[1]
d=json.load(sys.stdin)
roles=d.get(user,[])
names=[r['role_name'] for r in roles]
admin='system-admin' in names or 'lf-staff' in names
print(f'user: {user}')
print(f'LF admin (rolescopes-derived): {admin}  (system-admin: {\"system-admin\" in names}, lf-staff: {\"lf-staff\" in names})')
print('roles:', ', '.join(sorted(names)) or '(none)')
for r in roles:
    if r['role_name'] in ('cla-manager','cla-manager-designee','cla-signatory'):
        for s in r.get('scopes',[]):
            print(f'  {r[\"role_name\"]}: {s.get(\"object_type_name\")}/{s.get(\"object_id\")}')" "$USER_NAME"
    echo "note: gateway/warden may serve cached answers for up to ~10 min; use -H 'Cache-Control: no-cache' on API calls"
    ;;
  roles)
    USER_NAME="${1:-$DEFAULT_USER}"; TOKEN=$(get_token)
    rolescopes "$USER_NAME" | python3 -m json.tool
    ;;
  admin)
    ONOFF="${1:-}"; USER_NAME="${2:-$DEFAULT_USER}"; TOKEN=$(get_token)
    case "$ONOFF" in
      off)
        do_revoke system-admin '*' "$USER_NAME"
        do_revoke lf-staff '*' "$USER_NAME"
        wait_admin_state "$USER_NAME" false || true
        echo "admin OFF for $USER_NAME (restore with: $0 admin on $USER_NAME)"
        ;;
      on)
        do_grant system-admin admin '*' "$USER_NAME"
        do_grant lf-staff staff '*' "$USER_NAME"
        wait_admin_state "$USER_NAME" true || true
        echo "admin ON for $USER_NAME"
        ;;
      *) echo "usage: $0 admin on|off [username]" >&2; exit 1;;
    esac
    ;;
  grant)
    [ $# -ge 3 ] || { echo "usage: $0 grant <role> <object_type> <object_id> [username]" >&2; exit 1; }
    USER_NAME="${4:-$DEFAULT_USER}"; TOKEN=$(get_token)
    do_grant "$1" "$2" "$3" "$USER_NAME"
    ;;
  revoke)
    [ $# -ge 2 ] || { echo "usage: $0 revoke <role> <object_id> [username]" >&2; exit 1; }
    USER_NAME="${3:-$DEFAULT_USER}"; TOKEN=$(get_token)
    do_revoke "$1" "$2" "$USER_NAME"
    ;;
  warden)
    [ $# -ge 1 ] || { echo "usage: $0 warden <resource> [method] [username]" >&2; exit 1; }
    RES="$1"; METHOD="${2:-GET}"; USER_NAME="${3:-$DEFAULT_USER}"; TOKEN=$(get_token)
    warden_probe "$RES" "$METHOD" "$USER_NAME" | python3 -m json.tool
    ;;
  *)
    grep '^#   ' "$0" | sed 's/^#   //'
    exit 1
    ;;
esac
