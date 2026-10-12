# Tetris: playable CP/M showcase

Run in an ANSI/VT100 terminal:

```sh
bash /mnt/safe/minz/grace/2026-10-12/play-tetris/play.sh
```

Arrows move; up or lowercase z rotates; x drops; c holds; q exits. English keyboard layout. Ctrl-C stops the emulator and restores terminal mode. Gravity advances automatically: initially one row per eight 100 ms input ticks. The launcher supplies this emulator clock; ordinary CP/M has no standard wall clock and the COM alone still uses blocking BDOS1. Queued keys do not accelerate ticks, and arrow escape bytes belong to a single tick. Input echo is disabled.

The playable derivative redraws all 200 board cells, including empty cells, before drawing the moving piece. This removes trails and retains locked blocks. It also draws an ASCII border, score and cleared-line counter. Original corpus sources and earlier frozen evidence remain retained; source.nanz is a separate playable derivative.

Three-mode assembled Z80 render checks interpret actual ANSI output: after moving the I piece, old cells are black, current cells cyan, and a locked O cell yellow. Actual BDOS calls clobber registers; return PC/SP is checked. MZE tests cover idle ticks, queued keys, EOF, arrow sequences, standalone Escape, default echo and optional no-echo. A real MZE smoke kept stdin empty for 1.15 seconds, observed the cyan piece move from terminal row 2 to 3, then quit with status zero and empty stderr. This is a bounded emulator showcase, not proof of arbitrary play or physical CP/M timing.

Build from the compiler worktree root:

```sh
go -C minzc build -o reports/tetris-playable/mze ./cmd/mze
minzc reports/tetris-playable/source.nanz -t cpm --runtime cpm --runtime-stack 1024 --asserts none -o reports/tetris-playable/tetris.com
```

Put the freshly built mze beside play.sh and tetris.com. The shared package already contains all three. The previous launcher package is archived beside it before replacement.
