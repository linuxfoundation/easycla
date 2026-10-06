#!/bin/bash
if [ -z "$1" ]
then
  echo "Usage: $0 <branch-name>"
  exit 1
fi
echo "To be used when you are on a feature branch based on main and want to sync it with current $1 branch, so after merging current branch to main it will be the same as $1 but you can commit this once"

git fetch origin
git branch
git status
echo -n 'proceed (ctrl+c to stop)? '
read

# Exact copy of the $1 tree into index + worktree, deletions included
# (the former `git checkout $1 -- .` + `git add -A` kept files that $1 had deleted).
git restore --source="$1" --staged --worktree -- . || exit 1
if git diff --quiet "$1" -- && git diff --cached --quiet "$1" --; then
  echo "OK: index and worktree are identical to $1"
else
  echo "WARNING: still differs from $1:"; git diff --stat "$1" --; git diff --cached --stat "$1" --
fi

echo "Now you can do: git commit -S -asm 'msg'; git push"
