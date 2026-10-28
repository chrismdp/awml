#!/bin/bash
# Reads what one CI run's review and Classify jobs posted, from GitHub's record of the run: each job's
# own check on the commit, and the annotations it made there. Classify, fix.yml and land.yml run it
# from main's copy. The checks carry text from sessions that read a pull request's patch, so it's
# data: a finding counts only if its annotation's level and the start of its title agree on its
# rank, as the review job writes them, and Classify's verdict only if it's one of three words.
# GitHub gives an annotation that named no file the path .github, so such a finding has no file.
#
# Usage: read-findings.sh RUN DIR, where RUN is the CI run's id. It writes DIR/findings.json, a list
# of the findings of the run's Bug review and Security review, each with its review, severity, file,
# line, title and detail, and DIR/verdict.txt, Classify's verdict, ship, show or ask, or missing if
# its check gave none. Needs gh and jq, and a token that reads actions and checks.
set -euo pipefail

run="$1"
dir="$2"
case "$run" in ''|*[!0-9]*) echo "The run must be a number: $run" >&2; exit 1 ;; esac
mkdir -p "$dir"

gh api --paginate "repos/$GITHUB_REPOSITORY/actions/runs/$run/jobs?per_page=100" --jq '.jobs[]' |
  jq -s '[.[] | select(.name | IN("Bug review", "Security review", "Classify"))
    | {name, id: (.check_run_url | capture("/check-runs/(?<id>[0-9]+)$").id)}]' > "$dir/jobs.json"

annotations() {
  gh api --paginate "repos/$GITHUB_REPOSITORY/check-runs/$1/annotations?per_page=100" --jq '.[]' | jq -s .
}

echo '[]' > "$dir/findings.json"
while IFS=$'\t' read -r name id; do
  case "$id" in ''|*[!0-9]*) echo "The check of $name has no id." >&2; exit 1 ;; esac
  case "$name" in
    "Bug review") review=bug ;;
    "Security review") review=security ;;
    *) continue ;;
  esac
  annotations "$id" |
    jq --arg review "$review" --slurpfile all "$dir/findings.json" '$all[0] + [.[]
      | {failure: "Important", warning: "Nit", notice: "Pre-existing"}[.annotation_level] as $rank
      | select($rank != null and ((.title // "") | startswith($rank + ": ")))
      | (if (.path // "") == ".github" then {file: "", line: 0} else {file: (.path // ""), line: (.start_line // 0)} end) as $at
      | {review: $review, severity: $rank, file: $at.file, line: $at.line,
         title: (.title | ltrimstr($rank + ": ")), detail: (.message // "")}]' > "$dir/merged.json"
  mv "$dir/merged.json" "$dir/findings.json"
done < <(jq -r '.[] | [.name, .id] | @tsv' "$dir/jobs.json")

verdict=missing
classify="$(jq -r '[.[] | select(.name == "Classify")] | if length == 1 then .[0].id else "" end' "$dir/jobs.json")"
if [ -n "$classify" ]; then
  said="$(annotations "$classify" | jq -r '[.[] | select(.annotation_level == "notice" and .title == "Verdict")
    | .message] | if length == 1 then .[0] else "" end')"
  case "$said" in ship|show|ask) verdict="$said" ;; esac
fi
echo "$verdict" > "$dir/verdict.txt"
