#!/bin/sh
# Draw each example as Mermaid with awml, then as SVG with the Mermaid CLI, which needs Node and a Chrome.
# Point PUPPETEER_EXECUTABLE_PATH at your Chrome if Puppeteer can't find one.
#
# The drawings use Inter (examples/fonts, under the SIL Open Font License). Mermaid sizes each box
# by measuring its text in the Chrome that draws it, so that Chrome must have Inter: on Linux this
# script points fontconfig at examples/fonts, and on a Mac you install Inter-Regular.ttf first.
# Each SVG then carries the font itself, so it looks the same, and fits, wherever it's viewed.
set -eu
cd "$(dirname "$0")/.."
mkdir -p build/tmp
tmp=$(mktemp -d "$PWD/build/render.XXXXXX")
export TMPDIR="$PWD/build/tmp"
export XDG_CACHE_HOME="$tmp/cache"
export XDG_CONFIG_HOME="$tmp/config"
trap 'rm -rf "$tmp"' EXIT
go build -o "$tmp/draw" ./examples/render
# An explicit profile keeps Chrome's Unix socket path short inside a worktree.
printf '{"userDataDir":"%s/profile","args":["--no-sandbox","--disable-gpu","--disable-dev-shm-usage","--disable-crash-reporter"]}\n' "$tmp" > "$tmp/puppeteer.json"
if command -v fc-list >/dev/null && ! fc-list | grep -q 'Inter:style=Regular'; then
  cat > "$tmp/fonts.conf" <<CONF
<?xml version="1.0"?>
<fontconfig>
  <include ignore_missing="yes">/etc/fonts/fonts.conf</include>
  <dir>$PWD/examples/fonts</dir>
  <cachedir>$tmp/fontconfig</cachedir>
</fontconfig>
CONF
  FONTCONFIG_FILE="$tmp/fonts.conf"
  export FONTCONFIG_FILE
fi
printf '@font-face{font-family:"Inter";font-style:normal;font-weight:400;src:url(data:font/woff2;base64,%s) format("woff2")}\n' \
  "$(base64 < examples/fonts/Inter-Regular.woff2 | tr -d '\n')" > "$tmp/inter.css"
for f in examples/*.yaml examples/tour/*.yaml; do
  for drawing in $("$tmp/draw" "$f"); do
  npx -y @mermaid-js/mermaid-cli@11.4.1 -q -p "$tmp/puppeteer.json" -C "$tmp/inter.css" -b white -i "$drawing" -o "${drawing%.mmd}.svg"
  done
done
