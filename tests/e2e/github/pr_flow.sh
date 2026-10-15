#!/usr/bin/env bash
# Copyright The Linux Foundation and each contributor to CommunityBridge.
# SPDX-License-Identifier: MIT
# GitHub webhook pipeline, dev only (ALLOW_GITHUB_PR=1): a throw-away branch + commit authored as ID1's GitHub identity
# (author e-mail ID1_EMAIL -> GitHub attributes the commit to ID1_GITHUB) is pushed to GITHUB_REPO through the REST API
# (no clone), a PR is opened, and the EasyCLA app must post a commit status + a GITHUB_BOT comment within GH_TIMEOUT s.
# The verdict (success/failure) depends on ID1's authorization on the CLA group the repository is enrolled in - it is
# recorded as INFO; the checks assert that the pipeline answered, that the status and the comment agree, and that the
# legacy sign link resolves (302 to GitHub OAuth). The PR is closed and the branch deleted at the end unless --keep-state.
# Needs: gh (logged in as an account with push on GITHUB_REPO), curl, python3 - gh only once the module really runs.
set -u
. "$(cd "$(dirname "$0")/../lib" && pwd)/common.sh"
GH_TIMEOUT="${GH_TIMEOUT:-420}"

# every skip condition comes before the gh requirement: prod / --no-writes / gated runs must SKIP on a box without gh
if [ "$E2E_ENV" != "dev" ]; then e2e_result GH-PR-FLOW SKIP "GitHub PR flow runs on dev only (env=$E2E_ENV)"; e2e_summary_line; exit 0; fi
if ! e2e_gate ALLOW_GITHUB_PR GH-PR-FLOW "opening a PR on $GITHUB_REPO is disabled"; then e2e_summary_line; exit 0; fi
if [ "$E2E_NO_WRITES" = "1" ]; then e2e_result GH-PR-FLOW SKIP "--no-writes"; e2e_summary_line; exit 0; fi
if [ -z "${GITHUB_REPO:-}" ] || [ -z "${GITHUB_BOT:-}" ]; then e2e_result GH-PR-FLOW SKIP "GITHUB_REPO / GITHUB_BOT not configured"; e2e_summary_line; exit 0; fi
if ! command -v gh >/dev/null 2>&1; then e2e_result GH-PR-FLOW SKIP "gh CLI not installed"; e2e_summary_line; exit 0; fi

gh_user=$(gh api user --jq .login 2>/dev/null || true)
perm=$(gh api "repos/$GITHUB_REPO" --jq '.permissions.push' 2>/dev/null || echo false)
if [ "$perm" != "true" ]; then e2e_result GH-ACCESS SKIP "gh account ${gh_user:-?} has no push access to $GITHUB_REPO"; e2e_summary_line; exit 0; fi
e2e_result GH-ACCESS PASS "gh account $gh_user can push to $GITHUB_REPO"

default_branch=$(gh api "repos/$GITHUB_REPO" --jq .default_branch)
base_sha=$(gh api "repos/$GITHUB_REPO/git/ref/heads/$default_branch" --jq .object.sha)
branch="e2e-$E2E_ENV-$E2E_TAG"
pr_number=""
cleanup() {
  if [ -n "$pr_number" ] && [ "$E2E_KEEP_STATE" != "1" ]; then
    gh pr close "$pr_number" -R "$GITHUB_REPO" --delete-branch --comment "EasyCLA $E2E_ENV E2E suite $E2E_TAG: pipeline verified, closing." >/dev/null 2>&1 \
      && e2e_result GH-CLEANUP INFO "PR #$pr_number closed and branch $branch deleted (closed PR kept as proof)" \
      || e2e_result GH-CLEANUP INFO "PR #$pr_number could not be closed automatically - close it by hand"
  elif [ -n "$pr_number" ]; then
    e2e_result GH-CLEANUP INFO "--keep-state: PR #$pr_number left open on $GITHUB_REPO"
  elif [ "$E2E_KEEP_STATE" != "1" ]; then
    gh api -X DELETE "repos/$GITHUB_REPO/git/refs/heads/$branch" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

# ---- branch + commit attributed to ID1 (REST API: no clone, author e-mail = ID1_EMAIL) ----
if ! gh api -X POST "repos/$GITHUB_REPO/git/refs" -f ref="refs/heads/$branch" -f sha="$base_sha" >"$E2E_EVIDENCE_DIR/branch.private.json" 2>&1; then
  e2e_result GH-BRANCH FAIL "could not create branch $branch from $default_branch@${base_sha:0:7}: $(e2e_redact 200 < "$E2E_EVIDENCE_DIR/branch.private.json")"
  e2e_summary_line; exit 1
fi
content=$(printf 'EasyCLA %s E2E suite run %s - throw-away change, do not merge.\n' "$E2E_ENV" "$E2E_TAG" | base64 | tr -d '\n')
author_json=$(python3 -c 'import json,sys; print(json.dumps({"name": sys.argv[1], "email": sys.argv[2]}))' "$ID1_FULLNAME" "$ID1_EMAIL")
python3 -c 'import json,sys; print(json.dumps({"message": sys.argv[1], "content": sys.argv[2], "branch": sys.argv[3], "author": json.loads(sys.argv[4]), "committer": json.loads(sys.argv[4])}))' \
  "DEV EasyCLA E2E suite $E2E_TAG - do not merge" "$content" "$branch" "$author_json" >"$E2E_EVIDENCE_DIR/commit-request.private.json"
if ! gh api -X PUT "repos/$GITHUB_REPO/contents/e2e/$E2E_TAG.txt" --input "$E2E_EVIDENCE_DIR/commit-request.private.json" >"$E2E_EVIDENCE_DIR/commit.private.json" 2>&1; then
  e2e_result GH-COMMIT FAIL "contents API refused the commit: $(e2e_redact 200 < "$E2E_EVIDENCE_DIR/commit.private.json")"
  e2e_summary_line; exit 1
fi
head_sha=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["commit"]["sha"])' "$E2E_EVIDENCE_DIR/commit.private.json")
author_login=$(gh api "repos/$GITHUB_REPO/commits/$head_sha" --jq '.author.login // ""')
if [ "$author_login" = "$ID1_GITHUB" ]; then e2e_result GH-COMMIT-ATTRIBUTED PASS "commit ${head_sha:0:7} on $branch is attributed to $ID1_GITHUB"
else e2e_result GH-COMMIT-ATTRIBUTED FAIL "commit ${head_sha:0:7} attributed to '${author_login:-nobody}' (expected $ID1_GITHUB - is ID1_EMAIL a verified e-mail of that GitHub account?)"; fi

# ---- PR ----
pr_url=$(gh pr create -R "$GITHUB_REPO" --head "$branch" --base "$default_branch" --title "DEV EasyCLA E2E suite $E2E_TAG - do not merge" \
  --body "Automated EasyCLA $E2E_ENV E2E suite run $E2E_TAG (tests/e2e/github/pr_flow.sh). Exercises the GitHub webhook -> EasyCLA status/comment pipeline; closed automatically." 2>"$E2E_EVIDENCE_DIR/pr-create.err" || true)
pr_number=${pr_url##*/}
case "$pr_number" in ''|*[!0-9]*) e2e_result GH-PR-OPENED FAIL "gh pr create failed: $(e2e_redact 200 < "$E2E_EVIDENCE_DIR/pr-create.err")"; pr_number=""; e2e_summary_line; exit 1;; esac
opened_at=$(date +%s)
e2e_result GH-PR-OPENED PASS "$pr_url (head ${head_sha:0:7}, base $default_branch)"

# ---- wait for a TERMINAL EasyCLA status (success|failure|error; `pending` keeps polling) + the bot comment ----
status_state=""; status_desc=""; status_url=""; comment_body=""; status_at=""; comment_at=""; success_at=""
while :; do
  now=$(date +%s)
  case "$status_state" in success|failure|error) ;; *)
    gh api "repos/$GITHUB_REPO/commits/$head_sha/status" >"$E2E_EVIDENCE_DIR/status.private.json" 2>/dev/null || true
    eval "$(python3 - "$E2E_EVIDENCE_DIR/status.private.json" <<'PY'
import json, sys, shlex
try: d = json.load(open(sys.argv[1]))
except Exception: d = {}
s = next((x for x in d.get("statuses", []) if "easycla" in (x.get("context") or "").lower()), None)
if s:
    print("status_state=%s status_desc=%s status_url=%s status_at=%s" % (shlex.quote(s.get("state") or ""), shlex.quote((s.get("description") or "")[:120]), shlex.quote(s.get("target_url") or ""), shlex.quote(s.get("updated_at") or s.get("created_at") or "")))
PY
)"
  ;; esac
  if [ -z "$comment_body" ]; then
    gh api "repos/$GITHUB_REPO/issues/$pr_number/comments" >"$E2E_EVIDENCE_DIR/comments.private.json" 2>/dev/null || true
    eval "$(python3 - "$E2E_EVIDENCE_DIR/comments.private.json" "$GITHUB_BOT" <<'PY'
import json, sys, shlex
try: d = json.load(open(sys.argv[1]))
except Exception: d = []
bot = sys.argv[2]
c = next((x for x in d if (x.get("user") or {}).get("login") == bot), None)
if c:
    print("comment_body=%s comment_at=%s" % (shlex.quote(" ".join((c.get("body") or "").split())[:4000]), shlex.quote(c.get("created_at") or "")))
PY
)"
  fi
  # the backend (github_repository.go updatePullRequest) creates a comment only when a contributor is NOT authorized; on
  # success it merely edits an existing failure comment - so a success status may legitimately come with no comment:
  # give the comment a 60 s grace period after a success status and stop waiting
  case "$status_state" in
    success) if [ -n "$comment_body" ]; then break; fi; [ -z "$success_at" ] && success_at=$now; [ $((now - success_at)) -ge 60 ] && break ;;
    failure|error) [ -n "$comment_body" ] && break ;;
  esac
  [ $((now - opened_at)) -ge "$GH_TIMEOUT" ] && break
  sleep 15
done
elapsed=$(( $(date +%s) - opened_at ))
case "$status_state" in
  success|failure|error) e2e_result GH-EASYCLA-STATUS-POSTED PASS "EasyCLA status '$status_state | $status_desc' within ${elapsed}s (at $status_at)" "{\"state\":$(e2e_json_escape "$status_state"),\"seconds\":$elapsed}";;
  '') e2e_result GH-EASYCLA-STATUS-POSTED FAIL "no EasyCLA commit status on ${head_sha:0:7} after ${elapsed}s";;
  *) e2e_result GH-EASYCLA-STATUS-POSTED FAIL "EasyCLA commit status still '$status_state' (not terminal) after ${elapsed}s";;
esac
if [ -n "$comment_body" ]; then e2e_result GH-BOT-COMMENT PASS "$GITHUB_BOT commented at $comment_at: $(printf '%s' "$comment_body" | head -c 160)"
elif [ "$status_state" = success ]; then e2e_result GH-BOT-COMMENT INFO "no comment by $GITHUB_BOT on PR #$pr_number: the backend only comments when a contributor is not authorized (status is '$status_state')"
else e2e_result GH-BOT-COMMENT FAIL "no comment by $GITHUB_BOT on PR #$pr_number after ${elapsed}s"; fi
if [ -n "$status_state" ] && [ -n "$comment_body" ]; then
  # the comment's verdict: first the badge alt text the backend emits (getCommentBadge: "CLA Signed" vs "CLA Not Signed" /
  # "CLA Missing ID" / "CLA Confirmation Needed"), then the explicit sentences; negative phrases are matched before the
  # positive ones so "is not authorized under a signed CLA" can never count as agreement with a success status
  lc=$(printf '%s' "$comment_body" | tr 'A-Z' 'a-z')
  verdict=unknown
  if printf '%s' "$lc" | grep -Eq 'alt="cla (not signed|missing id|confirmation needed)"|is not authorized under a signed cla|please click here to be authorized|missing cla authorization'; then verdict=failure
  elif printf '%s' "$lc" | grep -Eq 'alt="cla signed"|is authorized under a signed cla|is covered by a signed cla|all contributors (are|have) (authorized|signed)'; then verdict=success; fi
  case "$status_state:$verdict" in
    success:success|failure:failure|error:failure) agree=1;;
    *) agree=0;;
  esac
  [ "$agree" = 1 ] && e2e_result GH-STATUS-COMMENT-AGREE PASS "status '$status_state' and the bot comment's verdict '$verdict' agree" || e2e_result GH-STATUS-COMMENT-AGREE FAIL "status '$status_state' vs comment verdict '$verdict': $(printf '%s' "$comment_body" | head -c 160)"
  printf '%s' "$lc" | grep -q "$(printf '%s' "$ID1_GITHUB" | tr 'A-Z' 'a-z')" && e2e_result GH-COMMENT-NAMES-AUTHOR PASS "bot comment names $ID1_GITHUB" || e2e_result GH-COMMENT-NAMES-AUTHOR INFO "bot comment does not name $ID1_GITHUB"
  e2e_result GH-VERDICT INFO "verdict '$status_state' is data-dependent: it reflects $ID1_GITHUB's authorization on the CLA group $GITHUB_REPO is enrolled in (not a pipeline property)"
fi
if [ -n "$status_url" ]; then
  code=$(curl -s -o /dev/null --max-time 60 -w '%{http_code}' "$status_url"); loc=$(curl -s -o /dev/null --max-time 60 -w '%{redirect_url}' "$status_url")
  case "$code" in
    30[1-8]) e2e_result GH-SIGN-LINK PASS "status target $(printf '%s' "$status_url" | sed -E 's#(https?://[^/]+/v2/repository-provider/github/sign/).*#\1…#') -> $code to ${loc%%\?*}";;
    200) e2e_result GH-SIGN-LINK PASS "status target -> 200 (console page)";;
    *) e2e_result GH-SIGN-LINK FAIL "status target -> $code";;
  esac
fi
e2e_summary_line
[ "$E2E_FAIL" -eq 0 ]
