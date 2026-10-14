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
# Keep code/data below the launcher's reserved 128-byte call stack.
python3 - "$TETRIS_OUT/program.bin" <<'PY_LAYOUT'
import pathlib, sys
size = pathlib.Path(sys.argv[1]).stat().st_size
if size == 0 or 0x6000 + size >= 0xfc60:
    raise SystemExit(f"Tetris binary overlaps reserved stack: {size} bytes at 6000")
print(f"Tetris: {size} bytes, code end {0x6000 + size:04X}, direct screen, reserved stack FC60..FCE0")
PY_LAYOUT
"$TETRIS_MZX" --model 48k --load "$TETRIS_OUT/program.bin@6000" --set PC=6000,SP=FCE0,DI,IM=1 --frames 100 --save-snapshot "$TETRIS_OUT/tetris.sna" --screenshot "$TETRIS_OUT/tetris.png"
