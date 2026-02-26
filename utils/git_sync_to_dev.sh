#!/bin/bash
echo 'To be used when you are on a feature branch based on main and want to sync it with current dev branch, so after merging current branch to main it will be the same as dev but you can commit this once'

git fetch origin
git branch
git status
echo -n 'proceed (ctrl+c to stop)? '
read

git rm -r --cached .
# git checkout origin/dev -- .
git checkout dev -- .
git add -A

echo "Now you can do: git commit -S -asm 'msg'; git push"
