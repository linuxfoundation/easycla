#!/bin/bash
# Check if all EasyCLA/LFX related repos (siblings of this repo, i.e. in
# github.com/linuxfoundation/) are "synced":
# - on their default branch (origin/HEAD),
# - no dirty/uncommitted/untracked files,
# - no pending push (commits ahead of origin),
# - no pending pull (commits behind origin).
# Special case: for 'easycla' both branches are checked: 'dev' (default,
# development) and 'main' (production).
# For every repo that is not synced it reports: wrong branch, dirty (with the
# list of files), missing push and/or missing pull.
# If a repo has no dirty files it is auto-synced: switched to its default
# branch and fast-forwarded with 'git pull --ff-only' (it never pushes).
# For 'easycla' both 'main' and 'dev' are pulled, ending on 'dev'.
# NOSYNC=1 - report only, don't switch branches and don't pull.
# NOFETCH=1 - skip 'git fetch' (faster, but pending pull info may be stale).
# REPOS='...' - override the default repos list.
set -u

base="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
repos="${REPOS:-easycla acs-cli lfx-corp-cla-console lfx-gateway lfx-v2-auth-service lfx-v2-persona-service traefik auth0-terraform easycla-contributor-console lfx-easycla-terraform lfx-gateway-terraform lfx-sanctions-screening lfx-self-serve lfx-v2-helm lfx-v2-project-service auth0-user-proxy easycla-landing-page lfx-pcc lfx-secrets-management lfx-self-serve-design-docs lfx-v2-argocd lfx-v2-opentofu}"

not_synced=0
checked=0

for repo in $repos
do
  dir="${base}/${repo}"
  if [ ! -d "$dir/.git" ]
  then
    echo "$repo: MISSING (no git repo at $dir)"
    not_synced=$((not_synced+1))
    continue
  fi
  checked=$((checked+1))
  problems=""
  actions=""

  if [ -z "${NOFETCH:-}" ]
  then
    git -C "$dir" fetch --quiet 2>/dev/null || problems="${problems}  - fetch failed (offline?), pull status may be stale\n"
  fi

  default_ref="$(git -C "$dir" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null)"
  if [ -z "$default_ref" ]
  then
    # try to set it from the remote, fall back to master/main
    git -C "$dir" remote set-head origin --auto >/dev/null 2>&1
    default_ref="$(git -C "$dir" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null)"
  fi
  default_branch="${default_ref#origin/}"
  if [ -z "$default_branch" ]
  then
    for b in master main
    do
      if git -C "$dir" show-ref --verify --quiet "refs/remotes/origin/$b"
      then
        default_branch="$b"
        break
      fi
    done
  fi
  if [ -z "$default_branch" ]
  then
    problems="${problems}  - cannot determine the default branch\n"
  fi

  # extra branches (besides the default one) to check/sync
  extra_branches=""
  if [ "$repo" = "easycla" ]
  then
    # easycla: 'dev' is the default/development branch, 'main' is production
    extra_branches="main"
  fi

  dirty="$(git -C "$dir" status --porcelain 2>/dev/null)"
  if [ -n "$dirty" ]
  then
    n="$(printf '%s\n' "$dirty" | wc -l | tr -d ' ')"
    problems="${problems}  - dirty ($n file(s)), auto-sync skipped:\n$(printf '%s\n' "$dirty" | sed 's/^/      /')\n"
  fi

  # auto-sync: only when there are no dirty files, ends on the default branch
  if [ -z "$dirty" ] && [ -z "${NOSYNC:-}" ] && [ -n "$default_branch" ]
  then
    for b in $extra_branches $default_branch
    do
      git -C "$dir" show-ref --verify --quiet "refs/remotes/origin/$b" || continue
      cur="$(git -C "$dir" rev-parse --abbrev-ref HEAD 2>/dev/null)"
      if [ "$cur" != "$b" ]
      then
        if git -C "$dir" checkout --quiet "$b" 2>/dev/null
        then
          actions="${actions}  * switched from '$cur' to '$b'\n"
        else
          problems="${problems}  - failed to switch from '$cur' to '$b'\n"
          continue
        fi
      fi
      before="$(git -C "$dir" rev-parse HEAD 2>/dev/null)"
      if git -C "$dir" pull --ff-only --quiet >/dev/null 2>&1
      then
        after="$(git -C "$dir" rev-parse HEAD 2>/dev/null)"
        if [ "$before" != "$after" ]
        then
          n="$(git -C "$dir" rev-list --count "${before}..${after}" 2>/dev/null)"
          actions="${actions}  * pulled $n commit(s) on '$b'\n"
        fi
      else
        problems="${problems}  - 'git pull --ff-only' failed on '$b' (diverged from origin/$b?)\n"
      fi
    done
  fi

  branch="$(git -C "$dir" rev-parse --abbrev-ref HEAD 2>/dev/null)"
  if [ -n "$default_branch" ] && [ "$branch" != "$default_branch" ]
  then
    problems="${problems}  - on branch '$branch' (default is '$default_branch')\n"
  fi

  for b in $default_branch $extra_branches
  do
    if ! git -C "$dir" show-ref --verify --quiet "refs/heads/$b"
    then
      problems="${problems}  - local branch '$b' is missing\n"
      continue
    fi
    if ! git -C "$dir" show-ref --verify --quiet "refs/remotes/origin/$b"
    then
      problems="${problems}  - no 'origin/$b' remote branch\n"
      continue
    fi
    counts="$(git -C "$dir" rev-list --left-right --count "origin/${b}...refs/heads/${b}" 2>/dev/null)"
    behind="$(echo "$counts" | awk '{print $1}')"
    ahead="$(echo "$counts" | awk '{print $2}')"
    [ -n "$ahead" ] && [ "$ahead" -gt 0 ] && problems="${problems}  - missing push: $ahead commit(s) ahead of origin/$b\n"
    [ -n "$behind" ] && [ "$behind" -gt 0 ] && problems="${problems}  - missing pull: $behind commit(s) behind origin/$b\n"
  done

  if [ -z "$problems" ]
  then
    if [ -z "$actions" ]
    then
      echo "$repo: OK"
    else
      echo "$repo: OK (auto-synced)"
      printf '%b' "$actions"
    fi
  else
    echo "$repo: NOT SYNCED"
    [ -n "$actions" ] && printf '%b' "$actions"
    printf '%b' "$problems"
    not_synced=$((not_synced+1))
  fi
done

echo
if [ "$not_synced" -eq 0 ]
then
  echo "All $checked repo(s) synced."
  exit 0
fi
echo "$not_synced repo(s) not synced."
exit 1
