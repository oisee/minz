# Next Session Seed — 2026-04-09

**Previous:** PBQP pivot, IRC client (Libera+DarkScience), port I/O, TUI View DSL proposal.
**Wisdom:** [contexts/2026-04-09-wisdom-irc-pbqp-ports.md](2026-04-09-wisdom-irc-pbqp-ports.md)

---

## Context

MinZ Z80 compiler. This mega-session (Apr 7-9) made the Z3→PBQP architecture pivot, built port I/O infrastructure, wrote an IRC client that connects to live servers, and designed a TUI View DSL. ~25 commits pushed.

## Current State

- **PBQP is production default** — `DefaultOptions()` no longer sets UseVIR
- **Port I/O works** — OpIn8/OpOut8/OpIn16/OpOut16 in MIR2, PortIO interface in VM, wired in MZV
- **IRC client connects** — Libera Chat (TCP) + DarkScience (TLS), PING/PONG works
- **IRC TUI is black screen** — network works, TUI host functions work individually, but full client exits immediately or shows nothing
- **TUI View DSL spec** — written by codex, reviewed by claude, frozen pending runtime fix
- **Both repos synced** — minz and minz-vir on same master

## Read First

1. `CLAUDE.md` — project overview
2. `reports/2026-04-09-Claude-IRC-TUI-Handoff-RU.md` — codex analysis of IRC TUI gap
3. `reports/2026-04-09-TUI-View-DSL-Proposal-RU.md` — TUI View DSL design
4. `reports/2026-04-09-Claude-TUI-View-DSL-Review.md` — review with Z80 lowering risks

## What To Do Next (in priority order)

### 1. Fix IRC client TUI runtime (black screen)

The minimal `test_irc_minimal.nanz` works perfectly — network, PING/PONG, data flow. The full `irc_client.nanz` with TUI clears screen and exits immediately.

**Debug approach:**
- `test_irc_tui_debug2.nanz` passes (set_str, status_msg, tui_* all work individually)
- `test_irc_tui_debug3.nanz` (loop + poll_network + poll_keyboard) sends NICK/USER but net_read returns 0
- Root cause hypothesis: stdin EOF in headless → exitCleanly fires, OR tight VM loop starves net readPump goroutine

**Files:** `cmd/mzv/main.go` (stdin reader), `cmd/mzv/port_host.go` (net_read host), `cmd/mzv/tui_host.go` (tui_read_key)

**Test command:**
```bash
timeout 15 go run ./cmd/mzv ../examples/nanz/test_irc_minimal.nanz --net irc.libera.chat:6667 --headless < /dev/zero
```

### 2. PBQP-PFCCO: close multi-call function gap

`OptimizeContractsPBQP` works for diamond graphs and pass-through chains but not for multi-call functions (e.g. `double_sum` calling `double` twice). `unaryCostWithMod` depends on original callee contracts via `inferNaturalClass`.

**Fix:** decouple `unaryCost` from original contracts — use candidate callee class instead.
**Guardrail tests:** `TestContractOptimize_PreservesOutput`, `TestContractScale_Long`
**File:** `pkg/mir2/contracts_pbqp.go`

### 3. TUI View DSL — implement Phase 1 (widget runtime)

Spec frozen. When ready, implement StatusBar/LogView/InputLine/NickList as plain Nanz functions (no metafunction yet). Add separator widget. Use dirty flags for redraw. Ring buffer for log.

### 4. `@inkey` / `@readkey` metafunction

Non-blocking key read that resolves per target: MZV→host function, CP/M→BDOS 6, ZX→port $FE, Agon→MOS. Currently `tui_read_key` but should be platform-abstracted.

### 5. Compile-time @extern check

`@extern` without host implementation crashes at runtime. Should warn at compile time (after all host registrations).

## What To Avoid

- Don't switch PBQP-PFCCO to default until multi-call gap closed
- Don't implement TUI View DSL until Phase 1 runtime is stable
- Don't redesign @screen — it works for forms, just needs live-app extension
- Don't touch Z3/VIR solver — parked

## Active Coordination

- `~/dev/minz-vir` — codex session, synced on same master
- Use `dedelulu explore` before sending — session IDs change
- Codex wrote TUI View DSL proposal + IRC TUI handoff analysis

## Key Test Commands

```bash
# IRC minimal (works):
timeout 15 go run ./cmd/mzv ../examples/nanz/test_irc_minimal.nanz --net irc.libera.chat:6667 --headless < /dev/zero

# IRC full TUI (black screen — needs fix):
go run ./cmd/mzv ../examples/nanz/irc_client.nanz --net irc.libera.chat:6667

# Fun showcase (29/29 compile):
cd minzc && for f in ../fun/*.nanz ../fun/*.frl; do go run ./cmd/minzc "$f" -o /dev/null 2>&1 | tail -1; done

# PBQP-PFCCO tests (14/14):
go test ./pkg/mir2/ -run "TestPBQP_PFCCO|TestContract" -count=1

# QBE tests (13/13):
go test ./pkg/mir2qbe/ -count=1
```

## Commits This Session (key ones)

| Hash | What |
|------|------|
| `49ea9e46` | fix: QBE u8 truncation |
| `8cb98773` | feat: PBQP-PFCCO + VIR sync |
| `fad66209` | feat: peephole conditional CALL |
| `ac462b4a` | feat: MIR2 OpCallCond |
| `cb20f806` | feat: OpIn8/OpOut8/OpIn16/OpOut16 |
| `e34c3174` | feat: MZV port infrastructure |
| `46d4ec14` | feat: IRC client v2 |
| `0a7dfc13` | feat: loop {} keyword |
| `31f31a72` | fix: PBQP default in pipeline |
| `6238cf6c` | feat: minimal IRC test (Libera Chat works!) |
| `706de703` | docs: TUI View DSL proposal + review |
