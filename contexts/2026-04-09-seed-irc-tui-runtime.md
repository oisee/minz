# Seed: 2026-04-09 IRC/TUI runtime fix in `minz`

## Working directory

Use:
- `/home/alice/dev/minz`

Do NOT switch back to `minz-vir` for this task.

## Goal

Make `examples/nanz/irc_client.nanz` render meaningfully and stay usable under `mzv`.

Do not redesign the TUI architecture in this task.
Do not build a new widget framework yet.

## Already proven

- `examples/nanz/test_irc_minimal.nanz` works against:
  - Libera Chat (TCP)
  - DarkScience (TLS)
- `PING/PONG` works.
- Raw TUI host functions work in isolation.
- `mzv` stdin/TUI path has already had partial fixes:
  - shared stdin reader
  - non-blocking `tui_read_key()`
  - EOF on stdin should not hard-exit headless/network apps

Therefore the remaining failure is in the full IRC/TUI path, not basic networking.

## First files to inspect

- `minzc/cmd/mzv/main.go`
- `minzc/cmd/mzv/tui_host.go`
- `minzc/cmd/mzv/port_host.go`
- `minzc/cmd/mzv/net_host.go`
- `examples/nanz/irc_client.nanz`
- `examples/nanz/test_irc_minimal.nanz`
- `examples/nanz/test_irc_tui_mock.nanz`

## Strong suspicions

1. `loop {}` in `irc_client.nanz`
- treat as suspicious
- if needed, replace main loop with `while 1 == 1` for a safer baseline

2. helper-heavy render path
- simplify these first if needed:
  - `show_raw`
  - `show_event`
  - `handle_privmsg`
  - `draw_input_line`
- avoid risky multi-arg helper calls if compiler/codegen still mishandles them

3. mock TUI path should be reusable
- if you use `test_irc_tui_mock.nanz`, keep it structurally close to the real IRC screen/update path
- goal is transport-swappable UI debugging, not a random separate demo

## Non-goals

- do not implement the full TUI View DSL
- do not redesign `@screen`
- do not broaden into general widget system work
- do not spend time on ZX/CP/M here

## Acceptance criteria

A good stop point is one of these:

### Preferred
- `irc_client.nanz` under `mzv` visibly renders status/log/input behavior and remains alive long enough to observe incoming IRC lines

### Acceptable intermediate
- a reusable mock-driven IRC TUI program renders correctly under `mzv`
- and the exact remaining delta to real `irc_client` is identified narrowly

## Verification

Suggested commands:

```bash
cd /home/alice/dev/minz/minzc

# Network sanity
 timeout 20 go run ./cmd/mzv ../examples/nanz/test_irc_minimal.nanz \
   --net irc.libera.chat:6667 --headless < /dev/zero

# TLS sanity
 timeout 30 go run ./cmd/mzv ../examples/nanz/test_irc_minimal.nanz \
   --net irc.darkscience.net:6697 --tls --headless < /dev/zero

# Full client
 go run ./cmd/mzv ../examples/nanz/irc_client.nanz --net irc.libera.chat:6667
```

## Reports worth rereading

- `reports/2026-04-09-Codex-IRC-MZV-Context.md`
- `reports/2026-04-09-TUI-View-DSL-Proposal-RU.md`
- `reports/2026-04-09-Claude-TUI-View-DSL-Review.md`

## Deliverable style

If you stop mid-debug, leave a short note with:
- exact current reproducer
- exact first broken function/path
- exact next recommended move
