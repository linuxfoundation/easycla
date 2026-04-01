#!/usr/bin/env python3
# source setenv.sh && cd cla-backend && source .venv/bin/activate && cd ..
# test those cases:
# Rebased PR: python3 utils/get_pr_commits.py mochajs mocha 5803
# Normal PR: python3 utils/get_pr_commits.py mochajs mocha 5686
# PR with >250 commits (2051): python3 utils/get_pr_commits.py mlehotskylf-org2 easycla-dev 30
# PR with >250 commits (301): python3 utils/get_pr_commits.py mlehotskylf-org2 easycla-dev 36
# PR co-authors: python3 utils/get_pr_commits.py mlehotskylf-org2 easycla-dev 45
# Other PRs from the above repo: 35, 29, 27, 26

import os
import sys
sys.path.insert(0, "./cla-backend")

from github import Github
from cla.models.github_models import (
    pygithub_graphql,
    get_pr_commit_count_gql,
    iter_pr_commits_compare,
)

def die(msg: str) -> None:
    print(f"ERROR: {msg}", file=sys.stderr)
    raise SystemExit(1)

def load_github_token() -> str:
    token_file = os.environ.get("GITHUB_TOKEN_FILE", "/etc/github/oauth").strip() or "/etc/github/oauth"
    try:
        with open(token_file, "r", encoding="utf-8") as f:
            token = f.read().strip()
    except OSError as exc:
        die(f"unable to read {token_file}: {exc}")
    if not token:
        die(f"{token_file} is empty")
    return token

def print_commitlite_list(title: str, commits):
    commits = list(commits)
    print(title)
    print(f"count: {len(commits)}")
    cc = 0
    for c in commits:
        first_line = (c.message or "").splitlines()[0] if c.message else ""
        print(
            c.sha,
            "| author_id:", c.author_id,
            "| login:", c.author_login,
            "| name:", c.author_name,
            "| email:", c.author_email,
            "| msg:", first_line,
        )
        cc += 1
    print(f"actual count: {cc}")
    print()
    return commits

if len(sys.argv) != 4:
    die(f"usage: {sys.argv[0]} <org> <repo> <pr_number>")

owner = sys.argv[1].strip()
repo_name = sys.argv[2].strip()

try:
    pr_number = int(sys.argv[3])
except ValueError:
    die(f"invalid pr_number: {sys.argv[3]!r}")

token = load_github_token()

g = Github(token)

query = """
query($owner:String!, $name:String!, $number:Int!) {
  repository(owner:$owner, name:$name) {
    pullRequest(number:$number) {
      commits {
        totalCount
      }
    }
  }
}
"""

data = pygithub_graphql(g, query, {"owner": owner, "name": repo_name, "number": pr_number})
print("GraphQL pullRequest.commits.totalCount:")
print(data)
print()

print("Helper get_pr_commit_count_gql:")
print(get_pr_commit_count_gql(g, owner, repo_name, pr_number))
print()

repo = g.get_repo(f"{owner}/{repo_name}")
pr = repo.get_pull(pr_number)

rest_commits = list(pr.get_commits())
print("REST totalCount:", pr.get_commits().totalCount)
cc = 0
for c in rest_commits:
    print(c.sha, c.commit.message.splitlines()[0])
    cc += 1
print("REST actual count:", cc)
print()

iter_compare_commits = print_commitlite_list(
    "iter_pr_commits_compare results:",
    iter_pr_commits_compare(g, owner, repo_name, pr_number),
)

rest_shas = [c.sha for c in rest_commits]
compare_shas = [c.sha for c in iter_compare_commits]

print("REST SHAs:                  ", rest_shas)
print("iter_pr_commits_compare SHAs:", compare_shas)
print()

print("REST vs compare:", [sha for sha in rest_shas if sha not in compare_shas])
print("compare vs REST:", [sha for sha in compare_shas if sha not in rest_shas])
