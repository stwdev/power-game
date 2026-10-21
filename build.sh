#!/bin/sh
# Builds the installable web app into dist/. Needs only Go (1.24+).
set -eu
cd "$(dirname "$0")"
OUT=${1:-dist}
rm -rf "$OUT"
mkdir -p "$OUT"
GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o "$OUT/game.wasm" ./cmd/wasm
GOROOT=$(go env GOROOT)
if [ -f "$GOROOT/lib/wasm/wasm_exec.js" ]; then
  cp "$GOROOT/lib/wasm/wasm_exec.js" "$OUT/"
else
  cp "$GOROOT/misc/wasm/wasm_exec.js" "$OUT/"
fi
cp web/index.html web/style.css web/app.js web/worker.js web/manifest.webmanifest \
   web/icon.svg web/icon-192.png web/icon-512.png web/icon-maskable-512.png "$OUT/"
# Stamp the service worker so each build replaces the offline cache.
VERSION=$( (cat "$OUT"/*.js "$OUT"/*.css "$OUT"/*.html "$OUT/game.wasm"; cat web/sw.js) | sha256sum | cut -c1-12)
sed "s/__VERSION__/$VERSION/" web/sw.js > "$OUT/sw.js"
echo "built $OUT (version $VERSION)"
