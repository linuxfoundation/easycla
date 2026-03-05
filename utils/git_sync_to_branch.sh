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

git rm -r --cached .
# git checkout origin/$1 -- .
git checkout $1 -- .
git add -A

echo "Now you can do: git commit -S -asm 'msg'; git push"
