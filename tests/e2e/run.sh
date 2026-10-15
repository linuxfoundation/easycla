#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
#
# EasyCLA end-to-end suite runner (manual, not CI). See tests/e2e/README.md.
#
# usage: tests/e2e/run.sh <dev|prod> [options]
#   --group <name>[,<name>...]  api | legacy | scans | browser | github | all (default: all)
#   --module <name>[,...]       run only these modules (file names without extension, e.g. health,read_matrix)
#   --no-writes                 skip every non-GET request against the environment (browser modules run read-only)
#   --keep-state                leave the fixture state the writes produced (default: restore the baseline)
#   --since <ISO-8601 UTC>      lambda log scan window start (default: run start minus 15 min; the scan also waits
#                               E2E_SCAN_SETTLE_SECS, default 90, for CloudWatch log ingestion)
#   --list                      print the modules per group and exit
#   --run-dir <dir>             evidence directory (default: e2e-tests/runs/suite/<env>-<UTC stamp>)
# exit code: 1 when any check FAILed.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_NAME="${1:-}"; shift || true
case "$ENV_NAME" in
  dev|prod) ;;
  *) sed -n '5,17p' "$0" >&2; exit 2 ;;
esac

GROUPS_SEL=all; MODULES_SEL=""; LIST=0; RUN_DIR=""
export E2E_ENV="$ENV_NAME" E2E_NO_WRITES=0 E2E_KEEP_STATE=0 E2E_SINCE=""
while [ $# -gt 0 ]; do
  case "$1" in
    --group) GROUPS_SEL="$2"; shift 2 ;;
    --module) MODULES_SEL="$2"; shift 2 ;;
    --no-writes) E2E_NO_WRITES=1; shift ;;
    --keep-state) E2E_KEEP_STATE=1; shift ;;
    --since) E2E_SINCE="$2"; shift 2 ;;
    --list) LIST=1; shift ;;
    --run-dir) RUN_DIR="$2"; shift 2 ;;
    *) echo "unknown option: $1" >&2; exit 2 ;;
  esac
done

# module registry: <group> <relative path> (order matters: tokens first, state-changing chains before the browser)
MODULES=(
  "api     api/health.sh"
  "api     api/hardened_route.sh"
  "api     api/read_matrix.py"
  "api     api/identity_boundary.py"
  "api     api/fixture_writes.py"
  "api     api/interactions.py"
  "legacy  legacy/legacy_api.sh"
  "browser browser/ld_flags.mjs"
  "browser browser/ss_identity.mjs"
  "browser browser/m3_org_lens.mjs"
  "browser browser/m3_chain.mjs"
  "browser browser/bridge.mjs"
  "browser browser/consoles.mjs"
  "browser browser/m2_signing.mjs"
  "browser browser/org_sign.mjs"
  "github  github/pr_flow.sh"
  "scans   scans/lambda_scan.sh"
)
if [ "$LIST" = 1 ]; then
  printf '%-8s %s\n' GROUP MODULE
  for m in "${MODULES[@]}"; do printf '%-8s %s\n' $m; done
  exit 0
fi

in_list() { local needle=$1 list=$2; [ "$list" = all ] && return 0; printf ',%s,' "$list" | grep -q ",$needle,"; }

export E2E_RUN_DIR="${RUN_DIR:-$ROOT/../../e2e-tests/runs/suite/$ENV_NAME-$(date -u +%Y%m%dT%H%M%SZ)}"
mkdir -p "$E2E_RUN_DIR"; chmod 700 "$E2E_RUN_DIR"
E2E_RUN_DIR="$(cd "$E2E_RUN_DIR" && pwd)"; export E2E_RUN_DIR
# E2E_SINCE stays empty unless --since was given: scans/lambda_scan.sh then scans from E2E_RUN_START minus 15 minutes
E2E_RUN_START="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
export E2E_RUN_START
export E2E_SINCE
chmod 700 "$ROOT/secrets" 2>/dev/null || true
echo "== EasyCLA E2E suite: env=$ENV_NAME groups=$GROUPS_SEL modules=${MODULES_SEL:-all} no-writes=$E2E_NO_WRITES keep-state=$E2E_KEEP_STATE"
echo "== evidence: $E2E_RUN_DIR"

# the selection must match something: a mistyped --module/--group would otherwise produce an empty "green" run
# (same group + module filters as the run loop); Node/Playwright are needed only when a browser module is selected
needs_node=0; selected=0
for m in "${MODULES[@]}"; do
  set -- $m
  in_list "$1" "$GROUPS_SEL" || continue
  if [ -n "$MODULES_SEL" ] && ! in_list "$(basename "${2%.*}")" "$MODULES_SEL"; then continue; fi
  selected=$((selected + 1))
  case "$2" in *.mjs) needs_node=1 ;; esac
done
if [ "$selected" = 0 ]; then
  echo "no module matched --group '$GROUPS_SEL' --module '${MODULES_SEL:-all}' (see --list)" >&2
  exit 2
fi

# tokens for both identities (cached while valid; refreshed again before each module)
E2E_MODULE=token "$ROOT/lib/token.sh" || { echo "token minting failed - aborting" >&2; exit 1; }

if [ "$needs_node" = 1 ]; then
  command -v node >/dev/null 2>&1 || { echo "node not found (browser group needs Node.js >= 20)" >&2; exit 1; }
  if [ ! -d "$ROOT/node_modules/playwright" ]; then
    echo "== installing playwright (npm install in tests/e2e)"
    (cd "$ROOT" && npm install --no-audit --no-fund >/dev/null) || { echo "npm install failed" >&2; exit 1; }
  fi
  # the chromium build matching the installed playwright version (no-op when already present)
  (cd "$ROOT" && npx --no-install playwright install chromium >/dev/null 2>&1) || echo "warning: 'npx playwright install chromium' failed - browser modules may not start" >&2
fi

rc=0
for m in "${MODULES[@]}"; do
  set -- $m; group=$1; rel=$2; name="$(basename "${rel%.*}")"
  in_list "$group" "$GROUPS_SEL" || continue
  if [ -n "$MODULES_SEL" ] && ! in_list "$name" "$MODULES_SEL"; then continue; fi
  echo; echo "== [$group] $rel"
  # dev tokens live one hour, a full run can be longer: re-check both tokens before every module (re-mints only
  # when less than 10 minutes are left; the "still valid - reusing" lines are kept out of the console)
  E2E_MODULE=token "$ROOT/lib/token.sh" | grep -v -- '- reusing$'
  [ "${PIPESTATUS[0]}" = 0 ] || echo "== warning: token refresh failed before $name - the module may see 401s" >&2
  export E2E_MODULE="$name"
  case "$rel" in
    *.sh)  bash "$ROOT/$rel" ;;
    *.py)  nice -n 10 python3 "$ROOT/$rel" ;;
    *.mjs) (cd "$ROOT" && NODE_OPTIONS="${NODE_OPTIONS:---max-old-space-size=768}" UV_THREADPOOL_SIZE="${UV_THREADPOOL_SIZE:-2}" nice -n 10 node "$rel") ;;
  esac || { echo "== module $name exited non-zero"; rc=1; }
done

echo
python3 "$ROOT/lib/summary.py" "$E2E_RUN_DIR" || rc=1
exit $rc
