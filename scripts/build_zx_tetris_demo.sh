#!/usr/bin/env bash
set -euo pipefail
TETRIS_ROOT=$(cd -- "$(dirname -- "$0")/.." && pwd)
TETRIS_OUT=${1:-"$TETRIS_ROOT/build/tetris-release"}
mkdir -p "$TETRIS_OUT"
TETRIS_MZ=${MZ:-"$TETRIS_ROOT/minzc/mz"}
TETRIS_MZA=${MZA:-"$TETRIS_ROOT/minzc/mza"}
TETRIS_MZX=${MZX:-"$TETRIS_ROOT/minzc/mzx-headless"}
"$TETRIS_MZ" "$TETRIS_ROOT/examples/zx/tetris_playable.nanz" -o "$TETRIS_OUT/body.a80"
cat "$TETRIS_ROOT/scripts/zx_tetris_entry.a80" "$TETRIS_OUT/body.a80" > "$TETRIS_OUT/program.a80"
"$TETRIS_MZA" "$TETRIS_OUT/program.a80" -o "$TETRIS_OUT/program.bin" -s "$TETRIS_OUT/program.sym"
"$TETRIS_MZX" --model 48k --load "$TETRIS_OUT/program.bin@8000" --set PC=8000,SP=FCE0,DI,IM=1 --frames 100 --save-snapshot "$TETRIS_OUT/tetris.sna" --screenshot "$TETRIS_OUT/tetris.png"
