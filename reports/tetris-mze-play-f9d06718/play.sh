#!/usr/bin/env bash
set -euo pipefail
play_dir=$(cd -- "$(dirname -- "$0")" && pwd)
play_tty_state=$(stty -g)
trap 'stty "$play_tty_state"; printf "\033[0m\033[?25h\n"' EXIT
stty -icanon -echo min 1 time 0
"$play_dir/mze" "$play_dir/tetris.com" -t cpm
