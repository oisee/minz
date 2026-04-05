# Next Session Seed

## Active Goal

Make `examples/nanz/tetris_cpm.nanz` render correctly under patched `mze`, after separating emulator input bugs from source/render bugs.

## Current Ground Truth

### In `minz-vir`

- [`stdlib/tui/render.nanz`](/home/alice/dev/minz-vir/stdlib/tui/render.nanz)
  - blocking `tui_read_key()`
  - VT100 arrow decode
  - broader `clob all`
- [`examples/nanz/tetris_cpm.nanz`](/home/alice/dev/minz-vir/examples/nanz/tetris_cpm.nanz)
  - currently back at last committed safe state
  - do not assume experimental local rewrites are still present
- [`reports/2026-04-05-Tetris-MZE-Progress-RU.md`](/home/alice/dev/minz-vir/reports/2026-04-05-Tetris-MZE-Progress-RU.md)
  - read first

### In sibling `minz`

There is a local uncommitted patch in:

- `/home/alice/dev/minz/minzc/cmd/mze/main.go`

It does:

- `BDOS 01` no fake `CR` on EOF
- `pendingInput` buffer
- `BDOS 0B` no longer loses char

This patch is compile-safe:

- `go test ./cmd/mze`
- `go run ./cmd/mze --help`

But it is not yet committed/pushed in `minz`.

## Proven Facts

1. `tui_goto` works.
   Hardcoded probe positions render correctly.

2. The remaining distortion is not fundamentally in TUI base rendering.

3. The remaining suspect zone is:
   - `piece_dx/piece_dy`
   - or arithmetic/call shape around `draw_cell`

4. Large direct-form rewrites of piece geometry are risky and can re-break `spawn_piece`.

## Required Approach

Do not directly rewrite the repo file in large chunks.

Use this workflow:

1. create probe copies in `build/` or `/tmp`
2. isolate one layer at a time
3. prove behavior with patched local `mze`
4. only then port the confirmed minimal fix into repo source

## Recommended Next Steps

### Step 1

Build two tiny probes from committed `tetris_cpm`:

- Probe A: hardcoded `draw_cell(...)`
- Probe B: hardcoded `tui_goto+tui_color+tui_putch`

Compare outputs.

Goal:

- determine whether `draw_cell(...)` itself is part of the problem.

### Step 2

If `draw_cell` is clean:

- probe arithmetic:
  - hardcoded `cx/cy`
  - then `cur_x/cur_y` arithmetic without piece LUT

Goal:

- identify whether `DRAW_X + cx * 2` / `DRAW_Y + cy` path is safe.

### Step 3

Only after that:

- isolate `piece_dx/piece_dy`
- preferably with one piece, one rotation, one cell at a time

## Non-goals

- no EXX
- no broad TUI redesign
- no full HUD restore yet
- no board render restore yet
- no big rewrite of all geometry logic in repo source

## Deliverables For Next Session

At minimum:

- one short report naming the exact remaining broken layer
- one minimal confirmed fix in repo source

Optional:

- commit in `minz` for the `mze` BDOS input fix if it is still the same patch
