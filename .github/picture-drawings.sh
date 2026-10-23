#!/bin/bash
# Redraws the examples a change alters, and pictures each drawing that changed, before and after.
# The build workflow's check job runs it on the patched tree, with the change staged and main's
# commit at HEAD.
#
# A drawing has changed when the change adds or alters its .mmd, since Claude regenerates the .mmd
# files and the checks hold them to awml draw. Only those SVGs are kept and pictured, so renderer
# noise in the others never reaches the change. A change to examples/render.sh or examples/render/
# changes the renderer itself, so every drawing is kept and pictured. A change that alters neither
# doesn't run the renderer at all.
#
# It writes into the directory it's given:
# - drawings.patch, the redrawn SVGs, kept apart from Claude's patch;
# - pictures/, a before and an after PNG of each changed SVG, with no before for a new one;
# - pictures/manifest.json, a list of {example, before, after}, paths relative to pictures/.
# A change that touches nothing drawn leaves the directory without them.
#
# Needs Node, a Chrome and jq. Point PUPPETEER_EXECUTABLE_PATH at the Chrome; this script uses the
# same one for the pictures.
set -euo pipefail
out="$1"
mkdir -p "$out"

renderer="$(git diff --cached --name-only HEAD -- \
  ':(glob)examples/render*' ':(glob)examples/render/**')"
if [ -n "$renderer" ]; then
  svgs=(':(glob)examples/*.svg' ':(glob)examples/tour/*.svg')
else
  svgs=()
  while IFS= read -r mmd; do
    svgs+=(":(literal)${mmd%.mmd}.svg")
  done < <(git diff --cached --name-only --diff-filter=AM HEAD -- \
    ':(glob)examples/*.mmd' ':(glob)examples/tour/*.mmd')
  if [ "${#svgs[@]}" -eq 0 ]; then
    echo "The change alters no example's .mmd and not the renderer, so nothing is redrawn."
    exit 0
  fi
fi

examples/render.sh
# Only the SVGs: the .mmd files are Claude's to regenerate, and the checks hold them to awml draw.
git add -N -- "${svgs[@]}"
git diff --binary -- "${svgs[@]}" > "$out/drawings.patch"
if [ ! -s "$out/drawings.patch" ]; then
  rm "$out/drawings.patch"
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
# GIF instead, but nothing in this repository moves yet.
pictures="$out/pictures"
mkdir -p "$pictures"
echo '[]' > "$pictures/manifest.json"
git diff --name-only --diff-filter=AM -- "${svgs[@]}" |
while IFS= read -r svg; do
  stem="${svg%.svg}"
  before=""
  if git cat-file -e "HEAD:$svg" 2> /dev/null; then
    before="$stem-before.png"
    git show "HEAD:$svg" > "$tmp/before.svg"
    picture "$tmp/before.svg" "$pictures/$before"
  fi
  after="$stem-after.png"
  picture "$svg" "$pictures/$after"
  jq --arg example "$svg" --arg before "$before" --arg after "$after" \
    '. + [{example: $example, before: $before, after: $after}]' "$pictures/manifest.json" \
    > "$tmp/manifest.json"
  mv "$tmp/manifest.json" "$pictures/manifest.json"
done
cat "$pictures/manifest.json"
