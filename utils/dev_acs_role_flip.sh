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
# Usage (mutating commands require an explicit username arg or ACS_USER env;
# read-only commands default to ACS_USER, then lgryglicki):
#   ./dev_acs_role_flip.sh status [username]
#   ./dev_acs_role_flip.sh roles [username]                 # full rolescopes JSON
#   ./dev_acs_role_flip.sh admin on|off <username>
#   ./dev_acs_role_flip.sh grant <role> <object_type> <object_id> <username>
#   ./dev_acs_role_flip.sh revoke <role> <object_id> <username>   # object_id '*' allowed
#   ./dev_acs_role_flip.sh warden <resource> [method] [username]  # raw warden probe
#   ./dev_acs_role_flip.sh orglens status [username]              # LFX One v2 Org Lens grants (b2b_org_settings)
#   ./dev_acs_role_flip.sh orglens on <orgSFID> <writer|auditor> <username>
#   ./dev_acs_role_flip.sh orglens off <orgSFID> <username>
#   ./dev_acs_role_flip.sh orglens verify <username>              # what the SS BFF role-grants query sees
#
# Examples - inspect:
#   ./dev_acs_role_flip.sh status lgryglicki                # role summary + cla-manager/designee/signatory scopes
#   ./dev_acs_role_flip.sh roles lgryglicki                 # full rolescopes JSON (all grant_ids)
#
# Examples - LF admin flip (system-admin + lf-staff together, losslessly):
#   ./dev_acs_role_flip.sh admin off lgryglicki
#   ./dev_acs_role_flip.sh admin on lgryglicki
#
# Examples - ACS role grants (object_id is '<projectSFID>|<orgSFID>'; dev fixtures:
# Sun project a09P000000DsCE5IAN, Infosys org 0014100000Te0G7AAJ, CNCF org 0014100000Te0yqAAB):
#   ./dev_acs_role_flip.sh grant cla-manager 'project|organization' 'a09P000000DsCE5IAN|0014100000Te0G7AAJ' lgryglicki
#   ./dev_acs_role_flip.sh grant cla-manager 'project|organization' 'a09P000000DsCE5IAN|0014100000Te0yqAAB' lgryglicki
#   ./dev_acs_role_flip.sh grant cla-manager-designee 'project|organization' 'a09P000000DsCE5IAN|0014100000Te0G7AAJ' lgryglicki
#   ./dev_acs_role_flip.sh grant cla-signatory 'project|organization' 'a09P000000DsCE5IAN|0014100000Te0G7AAJ' lgryglicki
#   ./dev_acs_role_flip.sh revoke cla-manager 'a09P000000DsCE5IAN|0014100000Te0G7AAJ' lgryglicki
#   ./dev_acs_role_flip.sh revoke cla-manager-designee '*' lgryglicki   # '*' = all scopes of that role
#
# Examples - warden probes (what the gateway authorizer would decide):
#   ./dev_acs_role_flip.sh warden /v4/company/external/0014100000Te0yqAAB/cla-groups GET
#   ./dev_acs_role_flip.sh warden /v4/signatures/project/01af041c-fa69-4052-a23c-fb8c1d3bef24/company/f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc/employee/csv GET lgryglicki
#
# Examples - LFX One v2 Org Lens (grants the /org/* pages' org selector; needs
# kubectl access to the lfx-dev EKS context, e.g. via awssso.sh):
#   ./dev_acs_role_flip.sh orglens status                   # membership on the fixture orgs (CNCF, Infosys)
#   ./dev_acs_role_flip.sh orglens on 0014100000Te0G7AAJ writer lgryglicki    # full org-lens access
#   ./dev_acs_role_flip.sh orglens on 0014100000Te0yqAAB auditor lgryglicki   # read-only variant
#   ./dev_acs_role_flip.sh orglens verify lgryglicki        # run the exact SS BFF role-grants query
#   ./dev_acs_role_flip.sh orglens off 0014100000Te0yqAAB lgryglicki
#   ORGLENS_ORGS='0014100000Te0yqAAB 0014100000Te0G7AAJ' ./dev_acs_role_flip.sh orglens status
#
# Env overrides:
#   ACS_USER=<u>                default username for read-only commands
#   AWS_PROFILE_OVERRIDE=<p>    AWS profile (default lfproduct-dev; dev-account guard still applies)
#   AWS_REGION_OVERRIDE=<r>     AWS region for SSM (default us-east-1)
#   EXPECTED_AWS_ACCOUNT=<id>   expected AWS account for the guard
#   LFX_V2_KUBE_CONTEXT=<ctx>   kube context for orglens (default lfx-dev; *prod* refused)
#   ORGLENS_ORGS='<sfid>...'    orgs listed by 'orglens status'
#   ORGLENS_EMAIL=<e>           member email for 'orglens on' (default <username>@contractor.linuxfoundation.org)
#
# Safety: refuses to run unless the AWS profile resolves to the LF dev account
# (override with EXPECTED_AWS_ACCOUNT). M2M token is cached under
# $XDG_RUNTIME_DIR or ~/.cache in a mode-0700 dir, written atomically.
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
DEFAULT_USER="${ACS_USER:-lgryglicki}"   # read-only commands only
EXPECTED_AWS_ACCOUNT="${EXPECTED_AWS_ACCOUNT:-395594542180}"   # LF dev account
CACHE_DIR="${XDG_RUNTIME_DIR:-$HOME/.cache}/easycla-acs-flip"
TOKEN_CACHE="$CACHE_DIR/m2m_${PROFILE}_${REGION}_${STAGE}.token"

require_user() {  # explicit-arg-or-ACS_USER for mutating commands
  local u="${1:-${ACS_USER:-}}"
  if [ -z "$u" ]; then
    echo "mutating command: pass <username> explicitly or set ACS_USER" >&2
    exit 1
  fi
  echo "$u"
}

check_aws_account() {
  local acct
  acct=$(aws sts get-caller-identity --profile "$PROFILE" --region "$REGION" --query Account --output text)
  if [ "$acct" != "$EXPECTED_AWS_ACCOUNT" ]; then
    echo "refusing to run: AWS profile '$PROFILE' is account $acct, expected dev account $EXPECTED_AWS_ACCOUNT" >&2
    exit 1
  fi
}

ssm() {
  aws ssm get-parameter --profile "$PROFILE" --region "$REGION" \
    --name "cla-auth0-platform-$1-${STAGE}" --query Parameter.Value --output text
}

mint_token() {
  local url cid sec aud
  url=$(ssm url); cid=$(ssm client-id); sec=$(ssm client-secret); aud=$(ssm audience)
  CID="$cid" SEC="$sec" AUD="$aud" python3 -c 'import json,os;print(json.dumps({"grant_type":"client_credentials","client_id":os.environ["CID"],"client_secret":os.environ["SEC"],"audience":os.environ["AUD"]}))' \
    | curl -s --max-time 20 -X POST "$url" -H "Content-Type: application/json" --data-binary @- \
    | python3 -c "import json,sys;d=json.load(sys.stdin);tok=d.get('access_token') or sys.exit('token mint failed: '+str(d));print(tok)"
}

get_token() {
  check_aws_account
  if [ -f "$TOKEN_CACHE" ] && [ -n "$(find "$TOKEN_CACHE" -mmin -50 2>/dev/null)" ]; then
    cat "$TOKEN_CACHE"
    return
  fi
  local tok tmp
  tok=$(mint_token)
  mkdir -p "$CACHE_DIR"
  chmod 700 "$CACHE_DIR"
  tmp=$(mktemp "$CACHE_DIR/.m2m.XXXXXX")
  printf '%s\n' "$tok" > "$tmp"
  chmod 600 "$tmp"
  mv -f "$tmp" "$TOKEN_CACHE"
  echo "$tok"
}

acs_get()    { curl -sfS --max-time 20 -H "Authorization: Bearer $TOKEN" "$ACS$1"; }
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
    case "$code" in
      204) echo "revoke $1 grant $gid ($rest): HTTP 204";;
      404) echo "revoke $1 grant $gid ($rest): already gone (404 - ACS read lag)";;
      *) echo "revoke $1 grant $gid ($rest) failed: HTTP $code" >&2; return 1;;
    esac
  done <<< "$found"
}

warden_probe() {  # resource method user
  curl -s --max-time 20 -X POST "$ACS/warden/subjects/authorize/v2" \
    -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
    -d "{\"username\":\"$3\",\"resource\":\"$1\",\"action\":\"$2\"}"
}

admin_state() {  # user -> prints true/false; isAdmin comes from system-admin only
  rolescopes "$1" | python3 -c "
import json,sys
names=[r['role_name'] for r in json.load(sys.stdin).get(sys.argv[1],[])]
print('true' if 'system-admin' in names else 'false')" "$1"
}

wait_admin_state() {  # user expected(true|false)
  local state
  for _ in $(seq 1 10); do
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

# ── LFX One v2 Org Lens (dev v2 EKS cluster; NOT ACS) ───────────────────────
# The SS /org/* lens gates on b2b_org_settings membership (member-service) and
# the lf-staff FGA team - both live in the lfx-v2 dev cluster. Auth there is a
# Heimdall-issued PS256 JWT; we mint one with the cluster's signer key (cluster
# admin via kubectl) and call the member-service directly through port-forward.
V2_CTX="${LFX_V2_KUBE_CONTEXT:-lfx-dev}"
V2_PF_PIDS=()

v2_check() {
  case "$V2_CTX" in *prod*) echo "refusing to run against context '$V2_CTX'" >&2; exit 1;; esac
  kubectl --context "$V2_CTX" get ns member-service >/dev/null 2>&1 \
    || { echo "kubectl context '$V2_CTX' unusable (need lfx-dev EKS access: awssso.sh)" >&2; exit 1; }
  python3 -c 'import jwt' 2>/dev/null || { echo "python3 pyjwt required (pip install pyjwt cryptography)" >&2; exit 1; }
  mkdir -p "$CACHE_DIR"; chmod 700 "$CACHE_DIR"
}

v2_cleanup() {
  if [ "${#V2_PF_PIDS[@]}" -gt 0 ]; then kill "${V2_PF_PIDS[@]}" 2>/dev/null || true; fi
  V2_PF_PIDS=()
  rm -f "$CACHE_DIR/.hdr.$$"
}

v2_pf() {  # ns svc remote_port -> echoes local port
  local out port
  out=$(mktemp "$CACHE_DIR/.pf.XXXXXX")
  kubectl --context "$V2_CTX" port-forward -n "$1" "svc/$2" ":$3" > "$out" 2>/dev/null &
  V2_PF_PIDS+=("$!")
  for _ in $(seq 1 40); do
    port=$(sed -n 's/^Forwarding from 127\.0\.0\.1:\([0-9]*\).*/\1/p' "$out" | head -1)
    [ -n "$port" ] && { rm -f "$out"; echo "$port"; return 0; }
    sleep 0.5
  done
  rm -f "$out"; echo "port-forward to $1/$2 failed" >&2; return 1
}

v2_mint() {  # audience -> token on stdout (cached <10 min per audience+user)
  local aud="$1" hp kid key cache tmp
  cache="$CACHE_DIR/v2_${V2_CTX}_${aud}_${ORGLENS_USER}.token"
  if [ -f "$cache" ] && [ -n "$(find "$cache" -mmin -10 2>/dev/null)" ]; then cat "$cache"; return; fi
  hp=$(v2_pf lfx lfx-platform-heimdall 4457) || exit 1
  kid=$(curl -s --max-time 10 "http://127.0.0.1:$hp/.well-known/jwks" | python3 -c 'import json,sys;print(json.load(sys.stdin)["keys"][0]["kid"])') \
    || { echo "cannot read heimdall JWKS" >&2; exit 1; }
  key=$(kubectl --context "$V2_CTX" get secret heimdall-signer-cert -n lfx -o jsonpath='{.data.key\.pem}' | base64 -d)
  tmp=$(mktemp "$CACHE_DIR/.v2m.XXXXXX")  # mktemp creates 0600 - no umask window
  KEY="$key" KID="$kid" AUD="$aud" USERV="$ORGLENS_USER" python3 - << 'PYEOF' > "$tmp"
import jwt, os, time
now = int(time.time())
print(jwt.encode(
    {"principal": os.environ["USERV"], "email": os.environ.get("ORGLENS_EMAIL", ""),
     "iss": "heimdall", "aud": os.environ["AUD"], "sub": os.environ["USERV"],
     "iat": now, "exp": now + 900},
    os.environ["KEY"], algorithm="PS256", headers={"kid": os.environ["KID"]}), end="")
PYEOF
  mv -f "$tmp" "$cache"
  cat "$cache"
}

orglens_settings() {  # local_port org -> settings JSON + etag file
  curl -sfS --max-time 15 -H "Authorization: Bearer $V2_TOKEN" -D "$CACHE_DIR/.hdr.$$" \
    "http://127.0.0.1:$1/b2b_orgs/$2/settings"
}

orglens_show() {  # settings-json org user
  python3 -c "
import json,sys
d=json.loads(sys.argv[1] or '{}'); org=sys.argv[2]; user=sys.argv[3]
found=[]
for rel in ('writers','auditors'):
    for m in (d.get(rel) or []):
        who=m.get('username') or m.get('email','?')
        st=m.get('invite_status','')
        print(f'  {org} {rel[:-1]}: {who} ({m.get(\"email\",\"\")}, {st})')
        if user in (m.get('username'), m.get('email')): found.append(rel[:-1])
if not (d.get('writers') or d.get('auditors')): print(f'  {org}: no members')
print(f'  -> {user}: ' + (' + '.join(found) if found else 'NO org-lens grant') + f' on {org}')" "$1" "$2" "$3"
}

orglens_put() {  # local_port org role user email
  local cur etag body out code
  cur=$(orglens_settings "$1" "$2") || exit 1
  etag=$(sed -n 's/^[Ee]tag: *//p' "$CACHE_DIR/.hdr.$$" | tr -d '\r' | head -1)
  body=$(CUR="$cur" ROLE="$3" USERV="$4" EMAIL="$5" python3 -c "
import json,os
d=json.loads(os.environ['CUR'] or '{}')
rel='writers' if os.environ['ROLE']=='writer' else 'auditors'
entry={'email':os.environ['EMAIL'],'username':os.environ['USERV'],'name':os.environ['USERV'],'invited_as':os.environ['ROLE']}
for k in ('writers','auditors'):
    d[k]=[m for m in (d.get(k) or []) if os.environ['USERV'] not in (m.get('username'), m.get('email'))]
d[rel]=d.get(rel,[])+[entry]
print(json.dumps({'writers':d.get('writers',[]),'auditors':d.get('auditors',[])}))")
  out=$(curl -s --max-time 15 -X PUT "http://127.0.0.1:$1/b2b_orgs/$2/settings" \
    -H "Authorization: Bearer $V2_TOKEN" -H "Content-Type: application/json" \
    ${etag:+-H "If-Match: $etag"} -d "$body" -w $'\n%{http_code}')
  code=${out##*$'\n'}
  if [ "$code" = "412" ] && echo "$out" | grep -q "no settings record exists"; then
    # first write for this org - the empty-doc GET still returns an ETag, but If-Match must be omitted
    out=$(curl -s --max-time 15 -X PUT "http://127.0.0.1:$1/b2b_orgs/$2/settings" \
      -H "Authorization: Bearer $V2_TOKEN" -H "Content-Type: application/json" \
      -d "$body" -w $'\n%{http_code}')
    code=${out##*$'\n'}
  fi
  case "$code" in
    200) echo "org-lens $3 set: $4 on $2";;
    *) echo "org-lens PUT $2 failed: HTTP $code"; echo "$out" | head -c 400; echo; exit 1;;
  esac
}

orglens_delete() {  # local_port org user
  local cur etag body out code rc=0
  cur=$(orglens_settings "$1" "$2") || exit 1
  etag=$(sed -n 's/^[Ee]tag: *//p' "$CACHE_DIR/.hdr.$$" | tr -d '\r' | head -1)
  body=$(CUR="$cur" USERV="$3" python3 -c "
import json,os,sys
d=json.loads(os.environ['CUR'] or '{}')
out={k:[m for m in (d.get(k) or []) if os.environ['USERV'] not in (m.get('username'), m.get('email'))] for k in ('writers','auditors')}
if out=={k:(d.get(k) or []) for k in ('writers','auditors')}: sys.exit(3)
print(json.dumps(out))") || rc=$?
  if [ "$rc" -eq 3 ]; then echo "no org-lens grant for $3 on $2"; return 0; fi
  [ "$rc" -eq 0 ] || exit 1
  # per-principal DELETE refuses to drop the last writer (last-Admin invariant); full-replace PUT can
  out=$(curl -s --max-time 15 -X PUT "http://127.0.0.1:$1/b2b_orgs/$2/settings" \
    -H "Authorization: Bearer $V2_TOKEN" -H "Content-Type: application/json" \
    ${etag:+-H "If-Match: $etag"} -d "$body" -w $'\n%{http_code}')
  code=${out##*$'\n'}
  case "$code" in
    200) echo "org-lens grant removed: $3 from $2";;
    *) echo "org-lens removal PUT $2 failed: HTTP $code" >&2; echo "$out" | head -c 400 >&2; echo >&2; exit 1;;
  esac
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
admin='system-admin' in names
print(f'user: {user}')
print(f'LF admin (isAdmin source = system-admin): {admin}  (lf-staff present: {\"lf-staff\" in names})')
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
    ONOFF="${1:-}"; USER_NAME=$(require_user "${2:-}"); TOKEN=$(get_token)
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
      *) echo "usage: $0 admin on|off <username>" >&2; exit 1;;
    esac
    ;;
  grant)
    [ $# -ge 3 ] || { echo "usage: $0 grant <role> <object_type> <object_id> <username>" >&2; exit 1; }
    USER_NAME=$(require_user "${4:-}"); TOKEN=$(get_token)
    do_grant "$1" "$2" "$3" "$USER_NAME"
    ;;
  revoke)
    [ $# -ge 2 ] || { echo "usage: $0 revoke <role> <object_id> <username>" >&2; exit 1; }
    USER_NAME=$(require_user "${3:-}"); TOKEN=$(get_token)
    do_revoke "$1" "$2" "$USER_NAME"
    ;;
  warden)
    [ $# -ge 1 ] || { echo "usage: $0 warden <resource> [method] [username]" >&2; exit 1; }
    RES="$1"; METHOD="${2:-GET}"; USER_NAME="${3:-$DEFAULT_USER}"; TOKEN=$(get_token)
    warden_probe "$RES" "$METHOD" "$USER_NAME" | python3 -m json.tool
    ;;
  orglens)
    SUB="${1:-}"; shift || true
    trap v2_cleanup EXIT
    case "$SUB" in
      status)
        ORGLENS_USER="${1:-$DEFAULT_USER}"
        v2_check; V2_TOKEN=$(v2_mint lfx-v2-member-service)
        MP=$(v2_pf member-service lfx-v2-member-service 8080)
        # default fixture orgs: CNCF, Infosys (or pass SFIDs via ORGLENS_ORGS)
        read -r -a ORGLIST <<< "${ORGLENS_ORGS:-0014100000Te0yqAAB 0014100000Te0G7AAJ}"
        for ORG in "${ORGLIST[@]}"; do
          orglens_show "$(orglens_settings "$MP" "$ORG")" "$ORG" "$ORGLENS_USER"
        done
        ;;
      on)
        [ $# -ge 3 ] || { echo "usage: $0 orglens on <orgSFID> <writer|auditor> <username>" >&2; exit 1; }
        ORG="$1"; ROLE="$2"; ORGLENS_USER=$(require_user "$3")
        case "$ROLE" in writer|auditor) ;; *) echo "role must be writer or auditor" >&2; exit 1;; esac
        EMAIL="${ORGLENS_EMAIL:-$ORGLENS_USER@contractor.linuxfoundation.org}"
        v2_check; V2_TOKEN=$(v2_mint lfx-v2-member-service)
        MP=$(v2_pf member-service lfx-v2-member-service 8080)
        orglens_put "$MP" "$ORG" "$ROLE" "$ORGLENS_USER" "$EMAIL"
        orglens_show "$(orglens_settings "$MP" "$ORG")" "$ORG" "$ORGLENS_USER"
        echo "note: SS BFF caches role-grants ~30s per user; UI needs a fresh page load after that"
        ;;
      off)
        [ $# -ge 2 ] || { echo "usage: $0 orglens off <orgSFID> <username>" >&2; exit 1; }
        ORG="$1"; ORGLENS_USER=$(require_user "$2")
        v2_check; V2_TOKEN=$(v2_mint lfx-v2-member-service)
        MP=$(v2_pf member-service lfx-v2-member-service 8080)
        orglens_delete "$MP" "$ORG" "$ORGLENS_USER"
        ;;
      verify)
        ORGLENS_USER="${1:-$DEFAULT_USER}"
        v2_check; V2_TOKEN=$(v2_mint lfx-v2-query-service)
        QP=$(v2_pf query-service lfx-v2-query-service 8080)
        # the exact query the SS BFF role-grants service runs for the org selector
        curl -sfS --max-time 15 -H "Authorization: Bearer $V2_TOKEN" \
          "http://127.0.0.1:$QP/query/resources?v=1&type=b2b_org_settings&tags=member:$ORGLENS_USER" \
          | USERV="$ORGLENS_USER" python3 -c "
import json,os,sys
user=os.environ['USERV']
d=json.load(sys.stdin)
rs=d.get('resources',[])
print(f'BFF role-grants query: {len(rs)} org(s) grant {user} org-lens access')
for r in rs:
    data=r.get('data') or {}
    roles={m.get('role') for m in (data.get('members') or []) if m.get('username')==user and m.get('invite_status')=='accepted'}
    for rel in ('writers','auditors'):
        if any(m.get('username')==user and m.get('invite_status')=='accepted' for m in (data.get(rel) or [])):
            roles.add(rel[:-1])
    org=str(r.get('id','')).split(':')[-1]
    print(f'  {org}: {\",\".join(sorted(roles)) or \"tagged but no accepted role\"}')"
        ;;
      *) echo "usage: $0 orglens status|on|off|verify ..." >&2; exit 1;;
    esac
    ;;
  *)
    grep '^#   [^ ]' "$0" | sed 's/^#   //'
    exit 1
    ;;
esac
