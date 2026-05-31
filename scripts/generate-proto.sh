#!/usr/bin/env sh
set -eu

ROOT="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
SOURCE_ROOT="${SOURCE_ROOT:-$(CDPATH= cd -- "$ROOT/.." && pwd)}"
COMMON_PROTO_DIR="${COMMON_PROTO_DIR:-$SOURCE_ROOT/common-lib/proto}"
PATH="$(go env GOPATH)/bin:$PATH"

rm -rf "$ROOT/gen"
mkdir -p "$ROOT/gen/go"

protoc -I "$ROOT/proto" -I "$COMMON_PROTO_DIR" \
  --go_out="$ROOT" \
  --go_opt=module=github.com/byte-v-forge/wa-app \
  --go-grpc_out="$ROOT" \
  --go-grpc_opt=module=github.com/byte-v-forge/wa-app \
  $(find "$ROOT/proto" -name '*.proto' | sort)

gofmt -w "$ROOT/gen/go"
