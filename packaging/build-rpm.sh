#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/nekssh-linux-amd64 .
"${NFPM:-$HOME/go/bin/nfpm}" package --config packaging/nfpm.yaml --packager rpm --target dist/
sha256sum dist/nekssh-*.rpm > dist/SHA256SUMS
