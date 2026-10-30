#!/bin/bash
# Reads an issue as the factory passes it on, keeping only what's approved. build.yml and fix.yml
# run it from main's copy, so both hold the same definition of approved:
# - the issue's title, labels and text count if its author has write access, or if the build label
#   was last added by someone with write access, or by the app's bot, and the issue hasn't been
#   edited or renamed since. A rename doesn't change GraphQL's lastEditedAt, so the issue's renamed
#   events count as edits too. Its author, who may be anyone, can edit it after the label, and
#   removing and adding the label again approves the text as it reads then (awml #120);
# - the comments of people with admin, maintain or write permission count, by the same may_write
#   as every workflow. Anyone can comment, so the text of strangers, and of members who can only
#   read, is never passed on (awml #5);
# - the app's own comments, its fixed text and the fixer's referrals, are model text no person
#   wrote, so one counts only if it was posted before the build label was last added and not
#   edited since.
# The label approves what the page shows, so the issue and its comments are read as GitHub renders
# them, bodyHTML, never the Markdown behind them, whose HTML comments the page hides (#150).
#
# Usage: issue-text.sh ISSUE FILE [WHY]. It writes what's approved of the issue, then each kept
# comment with its author, time and link, to FILE. If the build label doesn't approve the issue as
# it reads now, it writes why to WHY, if given, which is otherwise left empty: the build drafts only
# from an issue its label approves, whoever wrote it. Needs gh and jq, and a token that reads issues.
set -euo pipefail

number="$1"
file="$2"
why_file="${3:-/dev/null}"
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
      issue(number: $number) { number title bodyHTML lastEditedAt labels(first: 100) { nodes { name } } author { login __typename } }
    }
  }' --jq .data.repository.issue)"
remarks="$(mktemp)"
gh api graphql --paginate -F owner="$owner" -F name="$name" -F number="$number" -f query='
  query($owner: String!, $name: String!, $number: Int!, $endCursor: String) {
    repository(owner: $owner, name: $name) {
      issue(number: $number) {
        comments(first: 100, after: $endCursor) {
          nodes { author { login __typename } createdAt lastEditedAt url bodyHTML }
          pageInfo { hasNextPage endCursor }
        }
      }
    }
  }' --jq '.data.repository.issue.comments.nodes[]' | jq -s . > "$remarks"
events="$(gh api --paginate "repos/$GITHUB_REPOSITORY/issues/$number/events?per_page=100" \
  --jq '.[] | select((.event == "labeled" and .label.name == "build") or .event == "renamed")
    | {event, created_at, actor: {login: (.actor.login // ""), type: (.actor.type // "")}}' | jq -s .)"
labelled="$(jq -r '[.[] | select(.event == "labeled")] | last | .created_at // ""' <<< "$events")"
labeller="$(jq -r '[.[] | select(.event == "labeled")] | last | .actor.login // ""' <<< "$events")"
labeller_type="$(jq -r '[.[] | select(.event == "labeled")] | last | .actor.type // ""' <<< "$events")"
renamed="$(jq -r '[.[] | select(.event == "renamed")] | last | .created_at // ""' <<< "$events")"
edited="$(jq -r '.lastEditedAt // ""' <<< "$issue")"
if [[ "$renamed" > "$edited" ]]; then edited="$renamed"; fi
why=""
if ! { [ "$labeller" = "awml-factory[bot]" ] && [ "$labeller_type" = Bot ]; } && ! may_write "$labeller"; then
  why="The build label was last added by someone without write access, so it doesn't approve the issue."
elif [ -z "$labelled" ]; then
  why="The issue has never had the build label."
elif [ -n "$edited" ] && ! [[ "$edited" < "$labelled" ]]; then
  why="The issue was edited or renamed at $edited, after the build label was last added at $labelled, so it isn't drafted from."
fi
printf '%s' "$why" > "$why_file"
author="$(jq -r 'select(.author.__typename == "User") | .author.login // ""' <<< "$issue")"
approved=false
if [ -z "$why" ] || may_write "$author"; then approved=true; fi
# A deleted author has no login, so nobody's permission, and GraphQL names the app's bot by its slug
# with the type Bot, which no person's account has, with or without the [bot] the REST API adds.
writers=()
while read -r login; do
  if may_write "$login"; then writers+=("$login"); fi
done < <(jq -r '[.[] | select(.author.__typename == "User") | .author.login // empty] | unique[]' "$remarks")
{
  if [ "$approved" = true ]; then
    jq -r '"# #\(.number): \(.title)\n\nLabels: \([.labels.nodes[].name] | join(", "))\n\n\(.bodyHTML // "")\n"' <<< "$issue"
  else
    printf '# #%s\n\nThe issue'"'"'s title and text are left out: nobody with write access wrote them, or approved them as they read now with the build label. Only the comments below count.\n\n' "$number"
  fi
  jq -r --arg labelled "$labelled" --argjson writers "$(jq -nc '$ARGS.positional' --args "${writers[@]}")" \
    '.[] | select((.author.__typename == "User" and (.author.login | IN($writers[])))
        or (.author.__typename == "Bot" and (.author.login | IN("awml-factory", "awml-factory[bot]"))
          and $labelled != "" and .createdAt < $labelled and (.lastEditedAt // "") < $labelled))
      | "## Comment by \(.author.login), \(.createdAt), \(.url)\n\n\(.bodyHTML)\n"' "$remarks"
} > "$file"
