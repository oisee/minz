#!/usr/bin/env bash
set -euo pipefail
TETRIS_ROOT=$(cd -- "$(dirname -- "$0")/.." && pwd)
TETRIS_OUT=${1:-"$TETRIS_ROOT/build/tetris-video"}
TETRIS_GO=${GO:-go}
mkdir -p "$TETRIS_OUT" "$TETRIS_ROOT/build/tetris-recordings"
TETRIS_OUT=$(cd -- "$TETRIS_OUT" && pwd)
# Raw frames and verification stay local even when the requested output is a share.
TETRIS_CAPTURE=$(mktemp -d "$TETRIS_ROOT/build/tetris-recordings/capture.XXXXXX")
(
    cd "$TETRIS_ROOT/minzc"
    MINZ_TETRIS_VIDEO_LEVEL=4 MINZ_TETRIS_MAIN_VIDEO="$TETRIS_CAPTURE" "$TETRIS_GO" test ./pkg/pipeline \
        -run '^TestTetrisLineProgressionMainMachine/ordinary$' -v -count=1 \
        > "$TETRIS_CAPTURE/verification.txt" 2>&1
)
# Consecutive ULA frames, including repeats. No interpolation or time compression.
ffmpeg -hide_banner -loglevel error -y -framerate 50 -start_number 1 \
    -i "$TETRIS_CAPTURE/full-frames/frame_%04d.png" \
    -vf scale=704:592:flags=neighbor -c:v libx264 -pix_fmt yuv420p -crf 16 \
    "$TETRIS_OUT/tetris-session-50hz.mp4"
ffmpeg -hide_banner -loglevel error -y -framerate 50 -start_number 1 \
    -i "$TETRIS_CAPTURE/full-frames/frame_%04d.png" \
    -vf 'split[s0][s1];[s0]palettegen=max_colors=32[p];[s1][p]paletteuse=dither=none' \
    -loop 0 "$TETRIS_OUT/tetris-session-50hz.gif"
printf 'Videos: %s\nLocal frames and verification: %s\n' "$TETRIS_OUT" "$TETRIS_CAPTURE"
