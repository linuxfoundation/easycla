#!/bin/bash
echo 'To be used when you are on a feature branch based on dev and want to sync it with current main branch, so after merging current branch to dev it will be the same as main but you can commit this once'

git fetch origin
git branch
git status
echo -n 'proceed (ctrl+c to stop)? '
read

# Exact copy of the main tree into index + worktree, deletions included
# (replaces the destructive `git rm -rf .` + `git checkout main -- .` + `git add -A`).
git restore --source=main --staged --worktree -- . || exit 1
if git diff --quiet main -- && git diff --cached --quiet main --; then
  echo "OK: index and worktree are identical to main"
else
  echo "WARNING: still differs from main:"; git diff --stat main --; git diff --cached --stat main --
fi

echo "Now you can do: git commit -S -asm 'msg'; git push"
