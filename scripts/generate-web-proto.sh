#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_ROOT="${SOURCE_ROOT:-$(cd "${ROOT}/.." && pwd)}"
PROTO_DIR="${PROTO_DIR:-${ROOT}/proto}"
COMMON_PROTO_DIR="${COMMON_PROTO_DIR:-${SOURCE_ROOT}/common-lib/proto}"
OUT_DIR="${OUT_DIR:-${ROOT}/webui/src/proto}"
LOCAL_PLUGIN="${ROOT}/webui/node_modules/.bin/protoc-gen-ts_proto"
COMMON_PLUGIN="${SOURCE_ROOT}/common-lib/ui/node_modules/.bin/protoc-gen-ts_proto"
AGGREGATE_PLUGIN="${SOURCE_ROOT}/webui/node_modules/.bin/protoc-gen-ts_proto"
PLUGIN="${PROTOC_GEN_TS_PROTO:-}"

if [[ -z "${PLUGIN}" ]]; then
  if [[ -x "${LOCAL_PLUGIN}" ]]; then
    PLUGIN="${LOCAL_PLUGIN}"
  elif [[ -x "${COMMON_PLUGIN}" ]]; then
    PLUGIN="${COMMON_PLUGIN}"
  elif [[ -x "${AGGREGATE_PLUGIN}" ]]; then
    PLUGIN="${AGGREGATE_PLUGIN}"
  fi
fi

if [[ -z "${PLUGIN}" || ! -x "${PLUGIN}" ]]; then
  printf 'ts-proto plugin not found; run npm install in wa-app/webui, common-lib/ui, or webui first\n' >&2
  exit 1
fi

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"

protoc -I "${PROTO_DIR}" -I "${COMMON_PROTO_DIR}" \
  --plugin="protoc-gen-ts_proto=${PLUGIN}" \
  --ts_proto_out="${OUT_DIR}" \
  --ts_proto_opt=onlyTypes=true,outputServices=none,esModuleInterop=true,useJsonWireFormat=true,snakeToCamel=false \
  --ts_proto_opt=Mbyte/v/forge/contracts/account/v1/account.proto=@byte-v-forge/common-ui/proto/byte/v/forge/contracts/account/v1/account \
  $(find "${PROTO_DIR}" -name '*.proto' | sort)
