#!/usr/bin/env bash
# Run the ROM/IRQ-verified Nanz library demo in MZX.
set -euo pipefail
ROTO_REPO=$(cd -- "$(dirname -- "$0")/.." && pwd)
ROTO_SNAPSHOT="$ROTO_REPO/reports/zx-rotozoomer-library/native/ordinary/running.sna"
if [[ ${1:-} == --headless ]]; then
    shift
    ROTO_BUILD_DIR="$ROTO_REPO/build"
    mkdir -p "$ROTO_BUILD_DIR"
    (cd "$ROTO_REPO/minzc" && go build -tags mzx_headless -o "$ROTO_BUILD_DIR/mzx-headless" ./cmd/mzx)
    exec "$ROTO_BUILD_DIR/mzx-headless" --model 48k --snapshot "$ROTO_SNAPSHOT" "$@"
fi
if [[ ! -x "$ROTO_REPO/minzc/mzx" ]]; then
    make -C "$ROTO_REPO/minzc" mzx
fi
exec "$ROTO_REPO/minzc/mzx" --model 48k --no-audio --snapshot "$ROTO_SNAPSHOT" "$@"
