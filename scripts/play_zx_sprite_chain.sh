#!/usr/bin/env bash
# Run the typed whole-sprite scene or the separate low-level PUSH demo.
set -euo pipefail
CHAIN_REPO=$(cd -- "$(dirname -- "$0")/.." && pwd)
CHAIN_KERNEL=compiled
if [[ ${1:-} == --push ]]; then CHAIN_KERNEL=push; shift; fi
CHAIN_SNAPSHOT="$CHAIN_REPO/reports/advanced-demos/typed-sprite-scene/running.sna"
if [[ $CHAIN_KERNEL == push ]]; then CHAIN_SNAPSHOT="$CHAIN_REPO/reports/hardware-sprite-chain/demo/push.sna"; fi
if [[ ${1:-} == --headless ]]; then
 shift
 mkdir -p "$CHAIN_REPO/build"
 (cd "$CHAIN_REPO/minzc" && go build -tags mzx_headless -o "$CHAIN_REPO/build/mzx-headless" ./cmd/mzx)
 exec "$CHAIN_REPO/build/mzx-headless" --model 48k --snapshot "$CHAIN_SNAPSHOT" "$@"
fi
if [[ ! -x "$CHAIN_REPO/minzc/mzx" ]]; then make -C "$CHAIN_REPO/minzc" mzx; fi
exec "$CHAIN_REPO/minzc/mzx" --model 48k --no-audio --snapshot "$CHAIN_SNAPSHOT" "$@"
