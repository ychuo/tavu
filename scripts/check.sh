#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
# A dedicated cache also avoids a stale host testdeps cache observed on the
# development machine. No global module/build cache is edited.
. ./scripts/mygo-env.sh
if [ -n "$(gofmt -l main.go internal tools/mygopatch)" ]; then
  echo 'Go source is not formatted; run gofmt -w main.go internal' >&2
  exit 1
fi
go vet ./...
go test ./...
CGO_ENABLED=0 go test ./...
go tool mygo vet .
mkdir -p build
go build -o build/tavu .
