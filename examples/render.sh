#!/bin/sh
# Draw each example as Mermaid with awml, then as SVG with the Mermaid CLI, which needs Node and a Chrome.
# Point PUPPETEER_EXECUTABLE_PATH at your Chrome if Puppeteer can't find one.
set -eu
cd "$(dirname "$0")/.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
go build -o "$tmp/awml" ./cmd/awml
echo '{"args":["--no-sandbox"]}' > "$tmp/puppeteer.json"
for f in examples/*.yaml examples/tour/*.yaml; do
  "$tmp/awml" draw "$f" > "${f%.yaml}.mmd"
  npx -y @mermaid-js/mermaid-cli@11.4.1 -q -p "$tmp/puppeteer.json" -b white -i "${f%.yaml}.mmd" -o "${f%.yaml}.svg"
done
