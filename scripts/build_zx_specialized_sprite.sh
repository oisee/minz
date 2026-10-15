#!/usr/bin/env bash
# Compile once, measure native alternatives, save the exact winning code as SNA.
set -euo pipefail
SPRITE_REPO=$(cd -- "$(dirname -- "$0")/.." && pwd)
SPRITE_GOAL=${1:-budget}
SPRITE_OUTPUT=${2:-"$SPRITE_REPO/build/specialized-sprite"}
SPRITE_BUDGET=${3:-32000}
case "$SPRITE_GOAL" in speed|size|budget) ;; *) echo 'Usage: build_zx_specialized_sprite.sh [size|speed|budget] [NEW_OUTPUT_DIR] [BUDGET_T]' >&2; exit 2 ;; esac
if [[ -e "$SPRITE_OUTPUT" ]]; then echo "Output directory must be new: $SPRITE_OUTPUT" >&2; exit 2; fi
if [[ -z ${MZ:-} ]]; then
    mkdir -p "$SPRITE_REPO/build"
    (cd "$SPRITE_REPO/minzc" && go build -o "$SPRITE_REPO/build/mz" ./cmd/minzc)
fi
mkdir -p "$SPRITE_OUTPUT"
SPRITE_ARGS=(specialize "$SPRITE_REPO/reports/advanced-demos/meta-specialization/source.nanz"
    --goal "$SPRITE_GOAL" --target zxspectrum --stdlib "$SPRITE_REPO/stdlib"
    --workload "$SPRITE_REPO/reports/advanced-demos/meta-specialization/workload.json"
    --evidence "$SPRITE_OUTPUT/evidence" -o "$SPRITE_OUTPUT/selected.sna")
if [[ "$SPRITE_GOAL" == budget ]]; then SPRITE_ARGS+=(--budget-tstates "$SPRITE_BUDGET"); fi
"${MZ:-$SPRITE_REPO/build/mz}" "${SPRITE_ARGS[@]}"
printf 'Playable snapshot: %s/selected.sna\nMeasured ASM/BIN and choice: %s/evidence/\n' "$SPRITE_OUTPUT" "$SPRITE_OUTPUT"
