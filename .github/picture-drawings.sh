#!/bin/bash
# Redraws the examples a change alters, and pictures, before and after, each drawing that looks
# different. Claude runs it in the build workflow's draft job, from the repository's root, on its
# change in the working tree, once the checks pass. Before is the drawing at HEAD, the main the
# draft started from.
#
# A drawing has changed when the change adds or alters its .mmd, since Claude regenerates the .mmd
# files and the checks hold them to awml draw. Only those SVGs are kept, and every other SVG is put
# back as it was, so renderer noise never reaches the change. A change to examples/render.sh or
# examples/render/ changes the renderer itself, so every drawing is kept. A change that alters
# neither doesn't run the renderer at all. A drawing whose before and after pictures are the same
# pixels is left out of the pictures, though its SVG stays in the change, so a change to the
# renderer shows only the drawings it really changed.
#
# The kept SVGs stay in the working tree, as part of the change. It writes into the directory it's
# given:
# - pictures/, a before and an after PNG of each kept SVG that looks different, with no before for
#   a new one;
# - pictures/manifest.json, a list of {example, before, after}, paths relative to pictures/, which
#   is empty when every kept SVG looks the same as before.
# A change that redraws nothing leaves the directory without them.
#
# Needs Node, a Chrome and jq. Point PUPPETEER_EXECUTABLE_PATH at the Chrome; this script uses the
# same one for the pictures.
set -euo pipefail
out="$1"
mkdir -p "$out"

drawings=(':(glob)examples/*.svg' ':(glob)examples/tour/*.svg')

# What the working tree changes from HEAD, new files included.
changed() {
  { git diff --name-only HEAD -- "$@"; git ls-files --others --exclude-standard -- "$@"; } | sort -u
}

renderer="$(changed ':(glob)examples/render*' ':(glob)examples/render/**')"
keep=()
if [ -z "$renderer" ]; then
  while IFS= read -r mmd; do
    [ -f "$mmd" ] && keep+=("${mmd%.mmd}.svg")
  done < <(changed ':(glob)examples/*.mmd' ':(glob)examples/tour/*.mmd')
  if [ "${#keep[@]}" -eq 0 ]; then
    echo "The change alters no example's .mmd and not the renderer, so nothing is redrawn."
    exit 0
  fi
fi

examples/render.sh

# With the renderer unchanged, only the drawings whose .mmd changed are kept.
redrawn=()
while IFS= read -r svg; do
  if [ -n "$renderer" ] || printf '%s\n' "${keep[@]}" | grep -qxF -- "$svg"; then
    redrawn+=("$svg")
  elif git cat-file -e "HEAD:$svg" 2> /dev/null; then
    git restore --source=HEAD --worktree -- "$svg"
  fi
done < <(changed "${drawings[@]}")
if [ "${#redrawn[@]}" -eq 0 ]; then
  echo "No drawing changed."
  exit 0
fi

chrome="${PUPPETEER_EXECUTABLE_PATH:-google-chrome}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Draws an SVG as a PNG at its own width, up to 1200 pixels, at twice the pixels so text stays sharp.
picture() {
  local svg="$1" png="$2" size width height
  size="$(grep -o -m 1 '<svg[^>]*>' "$svg")"
  width="$(sed -n 's/.* width="\([0-9.]*\)".*/\1/p' <<< "$size")"
  height="$(sed -n 's/.* height="\([0-9.]*\)".*/\1/p' <<< "$size")"
  read -r width height < <(awk -v w="${width:-1200}" -v h="${height:-800}" \
    'BEGIN { s = w > 1200 ? 1200 / w : 1; printf "%d %d\n", w * s + 0.999, h * s + 0.999 }')
  cp "$svg" "$tmp/drawing.svg"
  printf '<!doctype html><body style="margin:0;background:white"><img src="drawing.svg" style="display:block;width:%spx;height:%spx">\n' \
    "$width" "$height" > "$tmp/page.html"
  mkdir -p "$(dirname "$png")"
  "$chrome" --headless=new --no-sandbox --disable-gpu --disable-dev-shm-usage --hide-scrollbars \
    --user-data-dir="$tmp/profile" --force-device-scale-factor=2 \
    --window-size="$width,$height" --screenshot="$png" "file://$tmp/page.html" 2> /dev/null
}

# Each drawing that changed is pictured as a still. A page that moves would be recorded here as a
# GIF instead, but nothing in this repository moves yet. The same Chrome writes the same pixels as
# the same bytes, so two pictures whose files match are the same pixels.
pictures="$out/pictures"
mkdir -p "$pictures"
echo '[]' > "$pictures/manifest.json"
for svg in "${redrawn[@]}"; do
  stem="${svg%.svg}"
  before=""
  if git cat-file -e "HEAD:$svg" 2> /dev/null; then
    before="$stem-before.png"
    git show "HEAD:$svg" > "$tmp/before.svg"
    picture "$tmp/before.svg" "$pictures/$before"
  fi
  after="$stem-after.png"
  picture "$svg" "$pictures/$after"
  if [ -n "$before" ] && cmp -s "$pictures/$before" "$pictures/$after"; then
    echo "$svg looks the same as before, so it isn't pictured."
    rm "$pictures/$before" "$pictures/$after"
    continue
  fi
  jq --arg example "$svg" --arg before "$before" --arg after "$after" \
    '. + [{example: $example, before: $before, after: $after}]' "$pictures/manifest.json" \
    > "$tmp/manifest.json"
  mv "$tmp/manifest.json" "$pictures/manifest.json"
done
cat "$pictures/manifest.json"
