#!/usr/bin/env bash
# Run the decoder test suite inside the nextmgmt/next-decoder container,
# which provides Go + CGO + libhdf5. Pulls the image if not already present
# locally. Run from the host:
#
#   ./test.sh                 # unit + fixture tests (mage test)
#   ./test.sh db              # + live DB integration test (mage testdb)
#   ./test.sh go <args...>    # arbitrary 'go' invocation, e.g. ./test.sh go test -run Huffman ./pkg/...
#
# DECODER_TEST_DB_{HOST,USER,PASS,NAME} override where the DB test connects
# (default: a local disposable container); used by CI to point at the mysql
# service container instead. See pkg/testdata/hddemo_616_seed.sql.
set -euo pipefail

DECODER_DIR="$(cd "$(dirname "$0")" && pwd)"
IMAGE="${IMAGE:-nextmgmt/next-decoder:latest}"
# Persistent module/build cache so repeated runs do not re-download deps
CACHE_DIR="${CACHE_DIR:-$HOME/.cache/decoder_go_container}"
mkdir -p "$CACHE_DIR/gopath" "$CACHE_DIR/gocache"

if ! docker image inspect "$IMAGE" >/dev/null 2>&1; then
    echo "Image $IMAGE not found locally, pulling..."
    docker pull "$IMAGE"
fi

case "${1:-test}" in
    db)   cmd=(mage testdb) ;;
    go)   shift; cmd=(go "$@") ;;
    *)    cmd=(mage test) ;;
esac

# Forward DB connection overrides only if set in the host environment, so a
# disposable container (CI) or an alternate host can be targeted without
# touching the defaults baked into pkg/database_test.go.
env_args=()
for var in DECODER_TEST_DB DECODER_TEST_DB_HOST DECODER_TEST_DB_USER DECODER_TEST_DB_PASS DECODER_TEST_DB_NAME; do
    if [ -n "${!var:-}" ]; then
        env_args+=(-e "$var")
    fi
done

exec docker run --rm \
    -v "$DECODER_DIR":/decoder \
    -v "$CACHE_DIR/gopath":/root/go \
    -v "$CACHE_DIR/gocache":/root/.cache/go-build \
    "${env_args[@]}" \
    -w /decoder \
    "$IMAGE" "${cmd[@]}"
