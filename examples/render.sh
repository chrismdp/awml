#!/bin/sh
# Draw each example as Mermaid, then as SVG with the Mermaid CLI, which needs Node and a Chrome.
# Point PUPPETEER_EXECUTABLE_PATH at your Chrome if Puppeteer can't find one.
set -eu
cd "$(dirname "$0")/.."
config=$(mktemp)
trap 'rm -f "$config"' EXIT
echo '{"args":["--no-sandbox"]}' > "$config"
for f in examples/*.yaml; do
  python3 -m awml draw "$f" > "${f%.yaml}.mmd"
  npx -y @mermaid-js/mermaid-cli@11.4.1 -q -p "$config" -b white -i "${f%.yaml}.mmd" -o "${f%.yaml}.svg"
done
