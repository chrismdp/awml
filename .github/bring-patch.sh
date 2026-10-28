#!/bin/bash
# Brings in a pull request's patch against main for CI's review and Classify jobs, which run it
# from main's checkout, copied out first, since the patch may change this file. It reads the patch
# as data and runs none of it.
#
# It keeps main's review prompts in RUNNER_TEMP/prompts, and main's REVIEW.md and AGENTS.md in
# RUNNER_TEMP/review, before the patch is applied, so the patch can't rewrite what it's held to. The
# pull request's own ci.yml, which GitHub runs on pull_request, could, and that's accepted: only
# people with write access, who are trusted, can push a branch here, and the app never pushes
# .github/.
# The patch is GitHub's diff of HEAD_SHA, the pull request's head, against the commit of main the
# branch started from, so it applies there, and the working tree then holds the change, all but
# binary images and fonts, which are listed in RUNNER_TEMP/review/binary-files.md instead. A patch
# that touches Claude Code's own settings, .claude/, .mcp.json or a CLAUDE.md, is refused before
# any session starts, and no session loads settings or servers from the tree.
#
# It reads HEAD_SHA, PR_NUMBER, GH_TOKEN and GitHub's own variables from the environment, and writes the
# count of binary files the sessions can't read as `unread` to GITHUB_OUTPUT. Needs gh and jq.
set -euo pipefail

# A file's name comes from the patch, so it's printed JSON-escaped, where a line break in it can't
# start a workflow command of its own.
named() { jq -nr --arg name "$1" '$name | @json'; }

case "${PR_NUMBER:-}" in ''|*[!0-9]*) echo "The pull request's number must be a number: ${PR_NUMBER:-}"; exit 1 ;; esac
if ! [[ "$HEAD_SHA" =~ ^[0-9a-f]{40}$ ]]; then
  echo "The head commit isn't a commit id: $HEAD_SHA"
  exit 1
fi
review="$RUNNER_TEMP/review"
mkdir -p "$review" "$RUNNER_TEMP/prompts"
cp .github/review-bugs-prompt.md .github/review-security-prompt.md .github/classify-prompt.md "$RUNNER_TEMP/prompts/"
cp REVIEW.md AGENTS.md "$review/"
main="$(git rev-parse HEAD)"
compare="repos/$GITHUB_REPOSITORY/compare/$main...$HEAD_SHA"
base="$(gh api "$compare" --jq .merge_base_commit.sha)"
if ! [[ "$base" =~ ^[0-9a-f]{40}$ ]] || ! git merge-base --is-ancestor "$base" "$main"; then
  echo "The pull request doesn't start from a commit of main: $base"
  exit 1
fi
gh api "$compare" -H "Accept: application/vnd.github.diff" > "$review/change.patch"
# A symbolic link in the working tree could lead a session anywhere, /proc included.
if grep -qE '^(new file mode|new mode) 120000$|^index [0-9a-f]+\.\.[0-9a-f]+ 120000$' "$review/change.patch"; then
  echo "The patch adds or changes a symbolic link, which a review session could follow out of the tree, so it isn't reviewed."
  exit 1
fi
# The pull request's title and description as they are now, read from GitHub, not the event that
# started CI, which holds them as they were when the push was made.
gh api "repos/$GITHUB_REPOSITORY/pulls/$PR_NUMBER" --jq '"# \(.title)\n\n\(.body // "")"' > "$review/pull-request.md"
git checkout -q --detach "$base"
# Every path the patch touches: git's numstat gives each file's name, the new one for a
# rename or copy, and the old names come from the patch's own rename and copy lines.
# numstat marks a binary file with "-", and so does the section it sits in.
git apply --numstat -z "$review/change.patch" > "$review/numstat"
paths=()
binaries=()
while IFS= read -r -d '' record; do
  added="${record%%$'\t'*}"
  path="${record#*$'\t'}"
  path="${path#*$'\t'}"
  paths+=("$path")
  [ "$added" = "-" ] && binaries+=("$path")
done < "$review/numstat"
# Each binary file once, by its new name, before a renamed one's old name is added.
changed="${#binaries[@]}"
while IFS=$'\t' read -r kind old; do
  if [[ "$old" == \"* ]]; then
    echo "The patch renames or copies a file whose name git quotes, which the review can't check: $(named "$old")"
    exit 1
  fi
  paths+=("$old")
  [ "$kind" = binary ] && binaries+=("$old")
done < <(awk 'function flush() { for (i = 1; i <= n; i++) print (binary ? "binary" : "text") "\t" olds[i]; n = 0; binary = 0 }
  /^diff --git / { flush() }
  /^(rename|copy) from / { olds[++n] = substr($0, index($0, " from ") + 6) }
  /^Binary files .* differ$/ || /^GIT binary patch$/ { binary = 1 }
  END { flush() }' "$review/change.patch")
claudes='(^|/)\.claude(/|$)|(^|/)\.mcp\.json$|(^|/)CLAUDE(\.local)?\.md$'
refuse() {
  echo "The patch changes Claude Code's own settings, servers or instructions, which a review session would load, so it isn't reviewed:"
  grep -zE "$claudes" | jq -Rs 'split("\u0000")[] | select(. != "")'
  exit 1
}
if [ "${#paths[@]}" -gt 0 ] && printf '%s\0' "${paths[@]}" | grep -zqE "$claudes"; then
  printf '%s\0' "${paths[@]}" | refuse
fi
# GitHub's diff names a binary file without its contents, so git can't apply it, and the
# reviews can't read it. A file is binary to git if it holds one NUL byte, so only images
# and fonts outside .github/ are let through unread; any other binary file, WebAssembly
# included, since it runs, stops the review. Leave out each binary file's section, apply
# the rest, and tell the reviewers which binary files changed. Then check that every other
# file is staged, and refuse Claude's own files among what was staged too.
readable='\.(png|jpe?g|gif|webp|ico|ttf|otf|woff2?)$'
for binary in "${binaries[@]}"; do
  if ! [[ "$binary" =~ ^[A-Za-z0-9._/@+-]+$ ]]; then
    echo "The patch changes a binary file whose name holds more than letters, digits and ./@+-_, which the reviews would be told as is."
    exit 1
  fi
  if [[ "$binary" == .github/* ]] || ! [[ "$binary" =~ $readable ]]; then
    echo "The patch changes $(named "$binary"), which GitHub's diff shows only as binary, so the reviews couldn't read it."
    exit 1
  fi
done
awk 'function flush() { if (section != "" && !binary) printf "%s", section; section = ""; binary = 0 }
  /^diff --git / { flush() }
  { section = section $0 "\n" }
  /^Binary files .* differ$/ || /^GIT binary patch$/ { binary = 1 }
  END { flush() }' "$review/change.patch" > "$review/text.patch"
if [ -s "$review/text.patch" ]; then
  git apply --index "$review/text.patch"
fi
if git diff --cached --name-only --no-renames -z | grep -zqE "$claudes"; then
  git diff --cached --name-only --no-renames -z | refuse
fi
staged="$(git diff --cached --name-only --no-renames -z | tr '\0' '\n')"
for path in "${paths[@]}"; do
  for binary in "${binaries[@]}"; do
    [ "$path" = "$binary" ] && continue 2
  done
  if ! grep -qxF -- "$path" <<< "$staged"; then
    echo "The patch touches $(named "$path"), but it wasn't applied, so the reviews wouldn't see it."
    exit 1
  fi
done
# The count goes to the step's output, out of the sessions' reach. The list goes in a file of
# the workflow's own, apart from the description the author wrote, which could carry a list of its
# own (awml #153).
echo "unread=$changed" >> "$GITHUB_OUTPUT"
if [ "${#binaries[@]}" -gt 0 ]; then
  {
    printf '## Binary files, changed but not shown in the patch\n\n'
    printf -- '- %s\n' "${binaries[@]}"
  } > "$review/binary-files.md"
fi
