#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"
OUT=dist/nekssh-windows-x64
rm -rf "$OUT"
mkdir -p "$OUT/data"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$OUT/nekssh.exe" .
cp packaging/windows/start-nekssh.cmd packaging/windows/start-lan.cmd packaging/windows/README.txt "$OUT/"
: > "$OUT/data/known_hosts"
cd dist
rm -f nekssh-0.1.0-windows-x64.zip
zip -qr nekssh-0.1.0-windows-x64.zip nekssh-windows-x64
sha256sum nekssh-0.1.0-windows-x64.zip > SHA256SUMS.windows
