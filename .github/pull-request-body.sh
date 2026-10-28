#!/bin/bash
# Writes the description of a build's pull request, in Markdown, to standard output. The build
# workflow's open job runs main's copy when it opens the pull request. Nothing reads the
# description back: it carries no record, digest or marker.
#
# It's Claude's description of the change, what changed and the decisions the issue didn't settle,
# escaped: < escaped, mentions and closing keywords broken and cut to 20,000 characters, so it can't
# hide the rest, notify anyone or close another issue. Then the pictures, and the issue it closes.
#
# It reads, from the environment:
# - ISSUE, a number;
# - CHANGE_DIR, the draft's files: pull-request.md, or failing that message.md;
# - PICTURES, a file of example|before|after rows, PICTURES_LEFT_OUT, a file saying why the
#   pictures were left out, and MEDIA_URL, where the pictures are.
# The files may be missing.
#
# Needs jq.
set -euo pipefail

case "$ISSUE" in ''|*[!0-9]*) echo "The issue must be a number: $ISSUE" >&2; exit 1 ;; esac

# A closing keyword before an issue's number or link would close that issue when the pull request
# merges, so a zero-width space before the number breaks it, and only the build's own Closes #n
# closes anything. A mention is broken the same way.
neutralise='gsub("@(?=[A-Za-z0-9])"; "@​")
  | gsub("(?<keyword>\\b(?:close[sd]?|fix(?:e[sd])?|resolve[sd]?)\\b[:\\s]+(?:(?:[\\w.-]+/[\\w.-]+)?#|https?://[^\\s/]+/[\\w.-]+/[\\w.-]+/(?:issues|pull)/))(?=[0-9])"; "\(.keyword)​"; "i")'
escape() {
  jq -Rrs '.[0:20000] | gsub("<"; "&lt;") | '"$neutralise"
}

# With the argument title, it writes the pull request's title instead: Claude's first line, cut to
# 200 characters, neutralised the same way, since a squash makes it the subject of main's commit.
if [ "${1:-}" = title ]; then
  title="$(head -n 1 "$CHANGE_DIR/message.md" 2> /dev/null || true)"
  [ -n "$title" ] || title="Build #$ISSUE"
  jq -Rr '.[0:200] | '"$neutralise" <<< "$title"
  exit 0
fi

# With the argument message, it writes the build's commit message: Claude's, cut to 5,000
# characters and neutralised the same way, since a person's squash puts it in main's commit, then
# the issue it closes.
if [ "${1:-}" = message ]; then
  message="$(cat "$CHANGE_DIR/message.md" 2> /dev/null || true)"
  [ -n "$message" ] || message="Build #$ISSUE"
  jq -Rrs '.[0:5000] | '"$neutralise" <<< "$message"
  printf '\nCloses #%s\n' "$ISSUE"
  exit 0
fi

if [ -s "$CHANGE_DIR/pull-request.md" ]; then
  escape < "$CHANGE_DIR/pull-request.md"
else
  tail -n +2 "$CHANGE_DIR/message.md" 2> /dev/null | sed '/./,$!d' | escape
fi

if [ -n "${PICTURES:-}" ] && [ -s "$PICTURES" ]; then
  if grep -qvE '^[A-Za-z0-9._/-]+\|[A-Za-z0-9._/-]*\|[A-Za-z0-9._/-]+$' "$PICTURES"; then
    echo "The pictures' rows aren't plain paths." >&2
    exit 1
  fi
  echo
  echo "## Pictures"
  while IFS='|' read -r example before after; do
    echo
    printf '### `%s`\n\n' "$example"
    echo "| Before | After |"
    echo "| --- | --- |"
    if [ -n "$before" ]; then
      before_cell="![Before]($MEDIA_URL/$before)"
    else
      before_cell="New drawing"
    fi
    printf '| %s | ![After](%s/%s) |\n' "$before_cell" "$MEDIA_URL" "$after"
  done < "$PICTURES"
elif [ -n "${PICTURES_LEFT_OUT:-}" ] && [ -s "$PICTURES_LEFT_OUT" ]; then
  echo
  echo "## Pictures"
  echo
  printf 'The pictures were left out: %s\n' "$(cat "$PICTURES_LEFT_OUT")"
fi

echo
printf 'Closes #%s\n' "$ISSUE"
