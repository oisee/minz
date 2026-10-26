#!/usr/bin/env bash
set -euo pipefail
repo=$(cd "$(dirname "$0")/../.." && pwd)
target=${MINZ_FLAT24_CARGO_TARGET:-/tmp/minz-flat24-native-target}
probe=${MINZ_FLAT24_C_PROBE:-/tmp/minz-ez80-c-probe}
"$repo/research/agon-flat24/verify.sh"
go -C "$repo/minzc" test ./cmd/ez80-c-probe -count=1
go -C "$repo/minzc" build -o "$probe" ./cmd/ez80-c-probe
python3 "$repo/research/agon-flat24/c-differential.py" --runner "$target/debug/minz-flat24-runner" --mzc-driver "$probe" "$@"
