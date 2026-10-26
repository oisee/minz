#!/usr/bin/env bash
set -euo pipefail
repo=$(cd "$(dirname "$0")/../.." && pwd)
target=${MINZ_FLAT24_CARGO_TARGET:-/tmp/minz-flat24-native-target}
cargo build --locked --manifest-path "$repo/research/agon-flat24/native-runner/Cargo.toml" --target-dir "$target"
MINZ_EZ80_RUNNER="$target/debug/minz-flat24-runner" go -C "$repo/minzc" test ./pkg/ez80 -run 'TestFlat24|TestADLRegisterWidths' -count=1 -v
go -C "$repo/minzc" test ./pkg/hir ./pkg/nanz -run TestFlat24 -count=1 -v
