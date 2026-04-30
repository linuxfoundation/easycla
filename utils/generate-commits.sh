#!/usr/bin/env bash
# Example:
# ./generate-commits.sh 513 50 "Extra Person <extra-person@example.com>" "Another Test User <another-test-user@example.org>"
#
set -Eeuo pipefail

usage() {
  cat <<'USAGE'
Usage:
  generate-easycla-commits.sh <commit-count> [random-identity-count] ["Name <email>"]...

Examples:
  ./generate-easycla-commits.sh 513 50
  ./generate-easycla-commits.sh 513 25 "Jane Doe <jane@example.com>" "Robot QA <robot@example.net>"

Notes:
  - The script only stages and commits README.md.
  - It does not create, switch, or push branches.
  - Extra identities must be quoted as single shell arguments.
  - By default, git hooks are skipped. Set NO_VERIFY=0 to run hooks.
USAGE
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

is_uint() {
  [[ "${1:-}" =~ ^[0-9]+$ ]]
}

contains() {
  local needle="$1"
  shift || true

  local item
  for item in "$@"; do
    [[ "$item" == "$needle" ]] && return 0
  done

  return 1
}

looks_like_identity() {
  local ident="$1"
  [[ "$ident" == *" <"*">" ]] || return 1

  local email="${ident##*<}"
  email="${email%>}"

  [[ "$email" == *@* ]] || return 1
  [[ "$email" != *" "* ]] || return 1
}

IDENTITIES=()

add_identity() {
  local ident="$1"

  looks_like_identity "$ident" || die "Identity must look like: Name <email>; got: $ident"

  if ! contains "$ident" "${IDENTITIES[@]}"; then
    IDENTITIES+=("$ident")
  fi
}

rand_hex() {
  od -An -N4 -tx1 /dev/urandom | tr -d '[:space:]'
}

FIRST_NAMES=(
  Ada Grace Linus Ken Brian Dennis Margaret Barbara Donald Alan Edsger Guido
  Rob Pike Brendan Martin Niklaus Anita James Sophie Maria Tomasz Pawel Anna
  Julia Adrian Monika Alicja Marek Karolina Daniel Natalia
)

LAST_NAMES=(
  Stone River Novak Kowalski Nowak Turing Hopper Torvalds Kernighan Ritchie
  Hamilton Liskov Knuth Dijkstra vanRossum Pike Eich Fowler Wirth Borg
  Kaminski Zielinski Adams Carter Morgan Taylor Brown Smith Wilson
)

DOMAINS=(
  example.com example.org example.net test.invalid synthetic.local
  generated.invalid easycla-test.invalid
)

make_random_identity() {
  local n="$1"

  local first="${FIRST_NAMES[$((RANDOM % ${#FIRST_NAMES[@]}))]}"
  local last="${LAST_NAMES[$((RANDOM % ${#LAST_NAMES[@]}))]}"
  local domain="${DOMAINS[$((RANDOM % ${#DOMAINS[@]}))]}"
  local suffix
  suffix="$(rand_hex)"

  local first_lc
  local last_lc
  first_lc="$(printf '%s' "$first" | tr '[:upper:]' '[:lower:]')"
  last_lc="$(printf '%s' "$last" | tr '[:upper:]' '[:lower:]')"

  printf '%s %s <%s.%s.%03d.%s@%s>' \
    "$first" "$last" "$first_lc" "$last_lc" "$n" "$suffix" "$domain"
}

COAUTHORS=()

choose_coauthors() {
  local author="$1"
  local wanted="$2"

  COAUTHORS=()

  local candidates=()
  local ident

  for ident in "${IDENTITIES[@]}"; do
    [[ "$ident" == "$author" ]] && continue
    candidates+=("$ident")
  done

  local max="${#candidates[@]}"
  if (( max == 0 )); then
    return 0
  fi

  if (( wanted > max )); then
    wanted="$max"
  fi

  local picked
  while (( ${#COAUTHORS[@]} < wanted )); do
    picked="${candidates[$((RANDOM % max))]}"

    if ! contains "$picked" "${COAUTHORS[@]}"; then
      COAUTHORS+=("$picked")
    fi
  done
}

write_commit_message() {
  local commit_no="$1"
  local total="$2"
  local change_desc="$3"
  local msg_file="$4"

  {
    printf 'EasyCLA synthetic README toggle %03d/%03d\n' "$commit_no" "$total"
    printf '\n'
    printf 'Synthetic commit for EasyCLA author and co-author handling.\n'
    printf 'README.md change: %s one letter.\n' "$change_desc"
    printf '\n'

    local coauthor
    for coauthor in "${COAUTHORS[@]}"; do
      printf 'Co-authored-by: %s\n' "$coauthor"
    done
  } > "$msg_file"
}

if [[ $# -lt 1 || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

COMMIT_COUNT="$1"
shift

is_uint "$COMMIT_COUNT" || die "commit-count must be a positive integer"
(( COMMIT_COUNT > 0 )) || die "commit-count must be greater than zero"

RANDOM_IDENTITY_COUNT=25
if [[ $# -gt 0 && "${1:-}" =~ ^[0-9]+$ ]]; then
  RANDOM_IDENTITY_COUNT="$1"
  shift
fi

is_uint "$RANDOM_IDENTITY_COUNT" || die "random-identity-count must be a non-negative integer"

# Fixed identities requested by the test.
add_identity "Lukasz Gryglicki <lgryglicki@cncf.io>"
add_identity "Justyna Gryglicka <justacakala@o2.pl>"
add_identity "My Bot <my-bot@o2.pl>"

# User-provided identities.
for ident in "$@"; do
  add_identity "$ident"
done

# Generated random identities.
for ((i = 1; i <= RANDOM_IDENTITY_COUNT; i++)); do
  add_identity "$(make_random_identity "$i")"
done

git rev-parse --is-inside-work-tree >/dev/null 2>&1 || die "Not inside a git work tree"

START_BRANCH="$(git symbolic-ref --quiet --short HEAD)" || die "Detached HEAD; checkout a branch first"
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

[[ -f README.md ]] || die "README.md not found at repo root: $REPO_ROOT"
git ls-files --error-unmatch -- README.md >/dev/null 2>&1 || die "README.md exists but is not tracked by git"

if ! git diff --quiet --exit-code -- .; then
  die "Working tree has unstaged tracked changes. Commit or stash them first."
fi

if ! git diff --cached --quiet --exit-code -- .; then
  die "Index has staged changes. Commit or unstage them first."
fi

GIT_COMMITTER_NAME="${GIT_COMMITTER_NAME:-}"
if [[ -z "$GIT_COMMITTER_NAME" ]]; then
  GIT_COMMITTER_NAME="$(git config user.name 2>/dev/null || true)"
fi
[[ -n "$GIT_COMMITTER_NAME" ]] || GIT_COMMITTER_NAME="EasyCLA Generator"
export GIT_COMMITTER_NAME

GIT_COMMITTER_EMAIL="${GIT_COMMITTER_EMAIL:-}"
if [[ -z "$GIT_COMMITTER_EMAIL" ]]; then
  GIT_COMMITTER_EMAIL="$(git config user.email 2>/dev/null || true)"
fi
[[ -n "$GIT_COMMITTER_EMAIL" ]] || GIT_COMMITTER_EMAIL="easycla-generator@example.com"
export GIT_COMMITTER_EMAIL

COMMIT_FLAGS=(--quiet --no-gpg-sign)
if [[ "${NO_VERIFY:-1}" != "0" ]]; then
  COMMIT_FLAGS+=(--no-verify)
fi

MSG_FILE="$(mktemp "${TMPDIR:-/tmp}/easycla-commit-msg.XXXXXX")"
trap 'rm -f "$MSG_FILE"' EXIT

BASE_TS="$(date +%s)"

echo "Generating $COMMIT_COUNT commits on current branch: $START_BRANCH"
echo "Identity pool size: ${#IDENTITIES[@]}"
echo "README.md will be changed by one letter per commit."

for ((commit_no = 1; commit_no <= COMMIT_COUNT; commit_no++)); do
  CURRENT_BRANCH="$(git symbolic-ref --quiet --short HEAD)" || die "Detached HEAD during generation"
  [[ "$CURRENT_BRANCH" == "$START_BRANCH" ]] || die "Branch changed from $START_BRANCH to $CURRENT_BRANCH"

  author="${IDENTITIES[$(((commit_no - 1) % ${#IDENTITIES[@]}))]}"

  if (( commit_no % 2 == 1 )); then
    printf 'x' >> README.md
    change_desc="added"
  else
    truncate -s -1 README.md
    change_desc="removed"
  fi

  git add -- README.md

  coauthor_count="$((RANDOM % 11))"
  choose_coauthors "$author" "$coauthor_count"
  write_commit_message "$commit_no" "$COMMIT_COUNT" "$change_desc" "$MSG_FILE"

  commit_date="@$((BASE_TS + commit_no))"

  # First create the commit, then amend its main author as requested.
  GIT_AUTHOR_NAME="$GIT_COMMITTER_NAME" \
  GIT_AUTHOR_EMAIL="$GIT_COMMITTER_EMAIL" \
  GIT_AUTHOR_DATE="$commit_date" \
  GIT_COMMITTER_DATE="$commit_date" \
    git commit "${COMMIT_FLAGS[@]}" -F "$MSG_FILE"

  GIT_AUTHOR_DATE="$commit_date" \
  GIT_COMMITTER_DATE="$commit_date" \
    git commit --amend "${COMMIT_FLAGS[@]}" --no-edit --author="$author"

  if (( commit_no == 1 || commit_no == COMMIT_COUNT || commit_no % 25 == 0 )); then
    echo "Created/amended $commit_no/$COMMIT_COUNT commits"
  fi
done

echo
echo "Done. Created $COMMIT_COUNT commits on branch: $START_BRANCH"
echo "Inspect authors and co-authors with:"
echo "  git log --format='%h %an <%ae>%n%B' --max-count=10"
