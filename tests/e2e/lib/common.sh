#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
#
# Shared helpers for the E2E suite (sourced by run.sh and every bash module).
# Portable bash (GNU + BSD userland): no `date -d`, no `tail -1`, no `readlink -f`.
#
# Requires E2E_ENV (dev|prod) and E2E_RUN_DIR to be exported by run.sh. A module can also be
# run standalone: `E2E_ENV=dev ./api/health.sh` (then a run dir is created automatically).

[ -n "${E2E_COMMON_LOADED:-}" ] && return 0
E2E_COMMON_LOADED=1

E2E_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_ROOT="$(cd "$E2E_LIB_DIR/.." && pwd)"
E2E_REPO_ROOT="$(cd "$E2E_ROOT/../.." && pwd)"
E2E_SECRETS_DIR="${E2E_SECRETS_DIR:-$E2E_ROOT/secrets}"
E2E_ENV="${E2E_ENV:-}"
case "$E2E_ENV" in
  dev|prod) ;;
  *) echo "error: E2E_ENV must be dev or prod (got '$E2E_ENV')" >&2; return 1 2>/dev/null || exit 1 ;;
esac

E2E_MODULE="${E2E_MODULE:-$(basename "${0%.*}")}"
E2E_RUN_DIR="${E2E_RUN_DIR:-$E2E_REPO_ROOT/e2e-tests/runs/suite/$E2E_ENV-$(date -u +%Y%m%dT%H%M%SZ)}"
E2E_EVIDENCE_DIR="$E2E_RUN_DIR/$E2E_MODULE"
mkdir -p "$E2E_EVIDENCE_DIR"
chmod 700 "$E2E_RUN_DIR" "$E2E_EVIDENCE_DIR" 2>/dev/null || true
E2E_RESULTS="$E2E_RUN_DIR/results.jsonl"
E2E_LOG="$E2E_RUN_DIR/run.log"
E2E_NO_WRITES="${E2E_NO_WRITES:-0}"
E2E_KEEP_STATE="${E2E_KEEP_STATE:-0}"
E2E_TAG="${E2E_TAG:-$(date -u +%Y%m%d%H%M%S)}"

# ---- config: secrets/<env>.env.secret, KEY=VALUE file, parsed (never sourced) ----
E2E_CONFIG="$E2E_ROOT/secrets/$E2E_ENV.env.secret"
e2e_cfg_get() { sed -n "s/^$1=//p" "$E2E_CONFIG" | sed -n '$p' | tr -d '\r'; }
e2e_load_config() {
  local key value
  if [ ! -f "$E2E_CONFIG" ]; then
    echo "missing $E2E_CONFIG - copy tests/e2e/config/example.env there and fill it in (see tests/e2e/README.md)" >&2
    exit 2
  fi
  while IFS= read -r key; do
    [ -z "$key" ] && continue
    value="$(e2e_cfg_get "$key")"
    # the environment (run.sh flags or the caller) wins over the file
    if [ -z "${!key+x}" ]; then printf -v "$key" '%s' "$value"; export "${key?}"; fi
  done < <(sed -n 's/^\([A-Z][A-Z0-9_]*\)=.*/\1/p' "$E2E_CONFIG")
}
e2e_load_config

# ---- secrets: one value per file, never printed ----------------------------
e2e_secret_path() { printf '%s/%s.secret' "$E2E_SECRETS_DIR" "$1"; }
e2e_secret() { # e2e_secret <name> -> value on stdout (empty if missing)
  local f; f="$(e2e_secret_path "$1")"
  [ -f "$f" ] && tr -d '\r\n' < "$f"
}
e2e_require_secret() {
  local f; f="$(e2e_secret_path "$1")"
  [ -f "$f" ] || { echo "error: missing secret file $f (see $E2E_ROOT/secrets/README.md)" >&2; return 1; }
}
# token for an identity: auth0-<env>-<identity>.token.secret (minted by lib/token.sh)
e2e_token() { e2e_secret "auth0-$E2E_ENV-$1.token"; }

# ---- logging / results ------------------------------------------------------
e2e_now() { date -u +%Y-%m-%dT%H:%M:%SZ; }
e2e_log() { printf '%s [%s] %s\n' "$(e2e_now)" "$E2E_MODULE" "$*" | tee -a "$E2E_LOG"; }
e2e_redact() { # redact tokens / passwords in a stream and cap length
  tr -d '\n' | sed -E 's/eyJ[A-Za-z0-9_.-]{20,}/[TOKEN]/g' | head -c "${1:-160}"
}
e2e_json_escape() { python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$1"; }
E2E_PASS=0; E2E_FAIL=0; E2E_SKIP=0; E2E_INFO=0
# e2e_result <id> <PASS|FAIL|SKIP|INFO> <note> [extra-json-object]
e2e_result() {
  local id=$1 status=$2 note=${3:-} extra=${4:-\{\}}
  case "$status" in
    PASS) E2E_PASS=$((E2E_PASS+1));; FAIL) E2E_FAIL=$((E2E_FAIL+1));;
    SKIP) E2E_SKIP=$((E2E_SKIP+1));; INFO) E2E_INFO=$((E2E_INFO+1));;
  esac
  python3 - "$E2E_RESULTS" "$E2E_MODULE" "$E2E_ENV" "$id" "$status" "$note" "$extra" <<'PY'
import json, sys, time
path, module, env, cid, status, note, extra = sys.argv[1:8]
try: more = json.loads(extra) if extra else {}
except Exception: more = {"extraRaw": extra[:300]}
row = {"id": cid, "status": status, "module": module, "env": env, "at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "note": note[:400], **more}
with open(path, "a") as fh: fh.write(json.dumps(row) + "\n")
PY
  e2e_log "$status $id - $note"
}
e2e_summary_line() { e2e_log "module done: PASS=$E2E_PASS FAIL=$E2E_FAIL SKIP=$E2E_SKIP INFO=$E2E_INFO evidence=$E2E_EVIDENCE_DIR"; }

# ---- HTTP -------------------------------------------------------------------
# e2e_http <identity|anon> <METHOD> <url> [json-body] -> sets E2E_LAST_CODE (status) and E2E_HTTP_OUT (body file).
# Call it directly (not in $(...)) so both variables reach the caller.
E2E_HTTP_OUT=""
E2E_LAST_CODE=000
e2e_http() {
  local who=$1 method=$2 url=$3 body=${4:-} tok=() code name hdr=""
  name="$(printf '%s' "$method-$url" | tr -c 'A-Za-z0-9._-' '_' | tail -c 120)"
  E2E_HTTP_OUT="$E2E_EVIDENCE_DIR/http-$(date -u +%H%M%S)-$RANDOM-$name.out"
  if [ "$who" != "anon" ]; then
    local t; t="$(e2e_token "$who")"
    [ -n "$t" ] || { echo "error: no token for identity $who (run lib/token.sh)" >&2; E2E_LAST_CODE=000; : > "$E2E_HTTP_OUT"; return 0; }
    # the token never appears in curl's argv (ps / /proc): it goes through a 0600 header file (curl -H @file, >= 7.55)
    hdr="$(umask 077; mktemp "$E2E_EVIDENCE_DIR/.hdr.XXXXXX")"
    printf 'Authorization: Bearer %s\n' "$t" > "$hdr"
    tok=(-H "@$hdr")
  fi
  if [ -n "$body" ]; then
    code=$(curl -s --max-time 90 -o "$E2E_HTTP_OUT" -w '%{http_code}' -X "$method" "${tok[@]}" \
      -H 'Accept: application/json' -H 'Content-Type: application/json' -H 'Cache-Control: no-cache' -d "$body" "$url")
  else
    code=$(curl -s --max-time 90 -o "$E2E_HTTP_OUT" -w '%{http_code}' -X "$method" "${tok[@]}" \
      -H 'Accept: application/json' -H 'Cache-Control: no-cache' "$url")
  fi
  [ -n "$hdr" ] && rm -f "$hdr"
  chmod 600 "$E2E_HTTP_OUT" 2>/dev/null || true
  E2E_LAST_CODE=$code
}
# e2e_check <id> <identity|anon> <expected-codes "200" | "200|404" | "?"> <METHOD> <url> [body]
#   "?" records INFO with the observed code instead of PASS/FAIL.
e2e_check() {
  local id=$1 who=$2 exp=$3 method=$4 url=$5 body=${6:-} code status
  if [ "$E2E_NO_WRITES" = "1" ] && [ "$method" != "GET" ] && [ "$method" != "HEAD" ]; then
    # callers that inspect the answer afterwards must see SKIP, not the previous call's code/body
    E2E_LAST_CODE=SKIP; : > "$E2E_HTTP_OUT"
    e2e_result "$id" SKIP "--no-writes: $method ${url#"$API_V4"}"; return 0
  fi
  e2e_http "$who" "$method" "$url" "$body"; code=$E2E_LAST_CODE
  local snippet; snippet="$(e2e_redact 140 < "$E2E_HTTP_OUT" 2>/dev/null)"
  if [ "$exp" = "?" ]; then status=INFO
  elif printf '|%s|' "$exp" | grep -q "|$code|"; then status=PASS
  else status=FAIL; fi
  local p=${url#"$API_V4"}; p=${p#"$LEGACY_API"}; p=${p#"$USER_SERVICE"}
  e2e_result "$id" "$status" "[$who] $method $p -> $code (expected $exp)" \
    "{\"http\":$code,\"expected\":$(e2e_json_escape "$exp"),\"who\":$(e2e_json_escape "$who"),\"body\":$(e2e_json_escape "$snippet")}"
  E2E_LAST_CODE=$code
}
# gate helper: e2e_gate <VAR> <id> <reason> -> returns 1 (and records SKIP) when the gate is off
e2e_gate() {
  local var=$1 id=$2 reason=$3
  if [ "${!var:-0}" != "1" ]; then e2e_result "$id" SKIP "$reason ($var=${!var:-0} in secrets/$E2E_ENV.env.secret)"; return 1; fi
  return 0
}
e2e_require_tools() { local t; for t in curl python3 "$@"; do command -v "$t" >/dev/null 2>&1 || { echo "error: $t not found" >&2; exit 1; }; done; }
e2e_require_tools
