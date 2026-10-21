#!/usr/bin/env bash
set -euo pipefail

mkdir -p build
port=$((10000 + $$ % 20000))
log=build/workerd.log
npx --yes wrangler dev --config cmd/awml-wasm/workerd/wrangler.jsonc --ip 127.0.0.1 --port "$port" >"$log" 2>&1 &
server_pid=$!
trap 'kill "$server_pid" 2>/dev/null || true; wait "$server_pid" 2>/dev/null || true' EXIT

for _ in $(seq 1 80); do
  if response=$(curl --silent --fail --data-binary @examples/question.yaml "http://127.0.0.1:$port/" 2>/dev/null); then
    printf '%s\n' "$response"
    exit 0
  fi
  sleep 0.25
done

cat "$log" >&2
exit 1
