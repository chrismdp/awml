#!/bin/bash
# Reads an issue as the factory passes it on: its title, labels and text, and the comments of
# people with admin, maintain or write permission, by the same may_write as every workflow, and of
# the app's own bot, whose comments are its fixed text and the fixer's referrals. Anyone can comment,
# so the text of strangers, and of members who can only read, is never passed on. The label approves
# what the page shows, so the issue and its comments are read as GitHub renders them, bodyHTML,
# never the Markdown behind them, whose HTML comments the page hides. build.yml and fix.yml run it
# from main's copy.
#
# Usage: issue-text.sh ISSUE FILE [JSON]. It writes the issue, then each kept comment with its
# author, time and link, to FILE, and the issue's own fields, lastEditedAt among them, to JSON if
# given. Needs gh and jq, and a token that reads issues.
set -euo pipefail

number="$1"
file="$2"
fields="${3:-/dev/null}"
case "$number" in ''|*[!0-9]*) echo "The issue must be a number: $number" >&2; exit 1 ;; esac

may_write() {
  [[ "$1" =~ ^[A-Za-z0-9-]{1,39}$ ]] || return 1
  [ "$(gh api "repos/$GITHUB_REPOSITORY/collaborators/$1/permission" \
    --jq '(.permission | IN("admin", "write")) or (.role_name | IN("admin", "maintain", "write"))' 2> /dev/null)" = true ]
}

owner="${GITHUB_REPOSITORY%%/*}"
name="${GITHUB_REPOSITORY#*/}"
issue="$(gh api graphql -F owner="$owner" -F name="$name" -F number="$number" -f query='
  query($owner: String!, $name: String!, $number: Int!) {
    repository(owner: $owner, name: $name) {
      issue(number: $number) { number title bodyHTML lastEditedAt labels(first: 100) { nodes { name } } }
    }
  }' --jq .data.repository.issue)"
remarks="$(mktemp)"
gh api graphql --paginate -F owner="$owner" -F name="$name" -F number="$number" -f query='
  query($owner: String!, $name: String!, $number: Int!, $endCursor: String) {
    repository(owner: $owner, name: $name) {
      issue(number: $number) {
        comments(first: 100, after: $endCursor) {
          nodes { author { login __typename } createdAt url bodyHTML }
          pageInfo { hasNextPage endCursor }
        }
      }
    }
  }' --jq '.data.repository.issue.comments.nodes[]' | jq -s . > "$remarks"
# A deleted author has no login, so nobody's permission, and GraphQL names the app's bot by its slug
# with the type Bot, which no person's account has, with or without the [bot] the REST API adds.
writers=()
while read -r login; do
  if may_write "$login"; then writers+=("$login"); fi
done < <(jq -r '[.[] | select(.author.__typename == "User") | .author.login // empty] | unique[]' "$remarks")
{
  jq -r '"# #\(.number): \(.title)\n\nLabels: \([.labels.nodes[].name] | join(", "))\n\n\(.bodyHTML // "")\n"' <<< "$issue"
  jq -r --argjson writers "$(jq -nc '$ARGS.positional' --args "${writers[@]}")" \
    '.[] | select((.author.__typename == "User" and (.author.login | IN($writers[])))
        or (.author.__typename == "Bot" and (.author.login | IN("awml-factory", "awml-factory[bot]"))))
      | "## Comment by \(.author.login), \(.createdAt), \(.url)\n\n\(.bodyHTML)\n"' "$remarks"
} > "$file"
printf '%s\n' "$issue" > "$fields"
