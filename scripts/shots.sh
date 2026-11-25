#!/usr/bin/env bash
# Screenshot the site's user flow (screenshots/flow.mjs) into screenshots/flow:
# desktop, phone and short phone. Uses the dev server at the URL if one is up,
# otherwise starts one for the run and stops it after. Run as `make shots`.
# Exits non-zero if the page threw an error (flow.mjs says which).
set -euo pipefail

url="${1:-http://localhost:5173}"
root="$(cd "$(dirname "$0")/.." && pwd)"
out="$root/screenshots/flow"

cd "$root/screenshots"
# Playwright is pinned in screenshots/package.json (a dev tool, never shipped).
if ! npm ls playwright >/dev/null 2>&1; then
  npm ci --no-audit --no-fund
fi
# The headless Chrome that version of Playwright drives; a no-op once cached.
npx playwright install --only-shell chromium

server=""
log=""
stop() {
  if [ -n "$server" ]; then kill "$server" 2>/dev/null || true; fi
  if [ -n "$log" ]; then rm -f "$log"; fi
}
trap stop EXIT

if ! curl -fs -o /dev/null "$url/"; then
  if [ ! -x "$root/web/node_modules/.bin/vite" ]; then
    echo "shots: no dev server at $url and web/node_modules is missing; run: cd web && npm ci" >&2
    exit 1
  fi
  port="${url##*:}"
  port="${port%%/*}"
  log="$(mktemp)"
  echo "shots: starting the dev server on port $port"
  (cd "$root/web" && exec ./node_modules/.bin/vite --port "$port" --strictPort) >"$log" 2>&1 &
  server=$!
  for _ in $(seq 1 60); do
    if curl -fs -o /dev/null "$url/"; then break; fi
    if ! kill -0 "$server" 2>/dev/null; then
      echo "shots: the dev server did not start:" >&2
      cat "$log" >&2
      exit 1
    fi
    sleep 0.5
  done
  curl -fs -o /dev/null "$url/" || { echo "shots: the dev server did not answer at $url" >&2; exit 1; }
fi

# A fresh folder, so numbered shots from an earlier run never mix with these.
rm -rf "$out"
node flow.mjs "$url" "$out"
echo "shots: saved to screenshots/flow"
