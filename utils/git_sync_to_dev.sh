#!/bin/bash
echo 'To be used when you are on a feature branch based on main and want to sync it with current dev branch, so after merging current branch to main it will be the same as dev but you can commit this once'

git fetch origin
git branch
git status
echo -n 'proceed (ctrl+c to stop)? '
read

# Exact copy of the dev tree into index + worktree, deletions included
# (replaces the destructive `git rm -rf .` + `git checkout dev -- .` + `git add -A`).
git restore --source=dev --staged --worktree -- . || exit 1
if git diff --quiet dev -- && git diff --cached --quiet dev --; then
  echo "OK: index and worktree are identical to dev"
else
  echo "WARNING: still differs from dev:"; git diff --stat dev --; git diff --cached --stat dev --
fi

echo "Now you can do: git commit -S -asm 'msg'; git push"
