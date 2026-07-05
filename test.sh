#!/usr/bin/env bash
# Run the decoder test suite inside the duck-backend-test-base container,
# which provides Go + CGO + libhdf5. Run from the host:
#
#   ./test.sh                 # unit + fixture tests (mage test)
#   ./test.sh go <args...>    # arbitrary 'go' invocation, e.g. ./test.sh go test -run Huffman ./pkg/...
set -euo pipefail

DECODER_DIR="$(cd "$(dirname "$0")" && pwd)"
IMAGE="${IMAGE:-duck-backend-test-base:latest}"
# Persistent module/build cache so repeated runs do not re-download deps
CACHE_DIR="${CACHE_DIR:-$HOME/.cache/decoder_go_container}"
mkdir -p "$CACHE_DIR/gopath" "$CACHE_DIR/gocache"

case "${1:-test}" in
    go)   shift; cmd=(go "$@") ;;
    *)    cmd=(mage test) ;;
esac

exec docker run --rm \
    -v "$DECODER_DIR":/decoder \
    -v "$CACHE_DIR/gopath":/root/go \
    -v "$CACHE_DIR/gocache":/root/.cache/go-build \
    -w /decoder \
    "$IMAGE" "${cmd[@]}"
