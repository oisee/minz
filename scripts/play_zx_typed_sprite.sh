#!/usr/bin/env bash
# Whole typed Nanz sprite scene; snapshot contains the verified native code.
set -euo pipefail
SPRITE_REPO=$(cd -- "$(dirname -- "$0")/.." && pwd)
SPRITE_SNAPSHOT="$SPRITE_REPO/reports/advanced-demos/typed-sprite-scene/running.sna"
if [[ ${1:-} == --headless ]]; then
    shift
    if [[ -z ${MZX:-} ]]; then
        mkdir -p "$SPRITE_REPO/build"
        (cd "$SPRITE_REPO/minzc" && go build -tags mzx_headless -o "$SPRITE_REPO/build/mzx-headless" ./cmd/mzx)
    fi
    exec "${MZX:-$SPRITE_REPO/build/mzx-headless}" --model 48k --snapshot "$SPRITE_SNAPSHOT" "$@"
fi
if [[ -z ${MZX:-} && ! -x "$SPRITE_REPO/minzc/mzx" ]]; then make -C "$SPRITE_REPO/minzc" mzx; fi
exec "${MZX:-$SPRITE_REPO/minzc/mzx}" --model 48k --no-audio --snapshot "$SPRITE_SNAPSHOT" "$@"
