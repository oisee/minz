# Session Wisdom — 2026-04-07..09 — PBQP Pivot, IRC Client, Port I/O

## Architecture Decisions (settled)

1. **Z3 parked, PBQP production default.** `DefaultOptions()` no longer sets `UseVIR: true`. Z3 stays as `--vir` flag for research. Two independent agents (Claude + GPT-5.4) reached same conclusion independently.

2. **Port I/O is the right abstraction for MZV.** `PortIO` interface shared across MZV (VM), MZE (Z80 emu), MZX (ZX Spectrum). Same port numbers everywhere: $23=console, $25=stderr, $30=net data, $31=net control. Programs run identically on all runtimes.

3. **OpIn8/OpOut8 for 8-bit ports, OpIn16/OpOut16 for 16-bit port address.** OpIn16 = Z80 `IN A,(C)` with BC=full 16-bit address. Needed for AY ($FFFD/$BFFD) and bank switching ($7FFD). Data is always 8-bit.

4. **TUI View DSL needed for live apps.** Forms DSL (`@screen`) covers selection screens. IRC/monitor/console need middle layer: StatusBar, LogView, InputLine, NickList. Spec written, reviewed, frozen pending runtime stabilization.

## Gotchas (hard-won)

1. **`loop {}` was not a Nanz keyword.** Parser accepted it but didn't generate function body. Fixed: `parseLoop()` → `WhileStmt{Cond: BoolLitExpr{true}}`.

2. **MZV `tui_read_key` was BLOCKING.** `os.Stdin.Read()` in the host function blocked the entire VM main loop. Fixed: non-blocking read from `stdinCh` channel.

3. **Three goroutines competing for stdin.** `readInput` (ZX keyboard), `stdinCh` goroutine, `tui_read_key` all called `os.Stdin.Read`. Fixed: single stdin reader goroutine, all consumers read from `stdinCh`. ZX keyboard goroutine only starts in `--zx` mode.

4. **Ctrl+C doesn't generate SIGINT in raw mode.** Arrives as byte 0x03 via stdin. Signal handler never fires. Fixed: stdin reader handles 0x04 (ctrl+d) as exit. Ctrl+c passes to program.

5. **`@extern` without host implementation = runtime crash.** MZV doesn't check at compile time. When `net_read` @extern wasn't registered as host function → `extern has no host implementation` error. Fixed by `registerPortHosts()`. Should be compile-time warning.

6. **MIR2 VM strings are null-terminated, no length prefix.** Z80 ASM has `DB N, "string"` (Pascal-style), but VM `resolveSymbol` appends raw string + NUL. Don't add `str_ptr()` offset in VM code.

7. **QBE uses 32-bit `w` for all types.** u8 arithmetic can overflow without wrapping. Fixed: `defNarrow()` inserts `and 255` after overflow-capable ops. SSA-safe via fresh temp.

8. **PBQP-PFCCO pass-through chain gap.** Functions that only forward params to calls have zero unary cost for all classes. `inferNaturalClass` reads original callee contract (ClassGeneral), not optimized one. Fixed partially: `isPassThrough()` zeroes unary cost for simple forwarders. Multi-call functions still have bias.

9. **Conditional CALL pattern.** Z80 has `CALL cc, target` but no compiler used it. MinZ now detects `BrIf → single-call-block → join` at MIR2 level and emits `OpCallCond`. Peephole catches remaining `JR cc / CALL / label` patterns.

10. **IRC PING/PONG is mandatory.** Server sends `PING :token`, client must respond `PONG :token` (including the colon). Without it, disconnect after ~120s. Token is random each time.

## Key Files Modified

| File | What changed |
|------|-------------|
| `pkg/mir2/ops.go` | OpCallCond, OpIn8/OpOut8/OpIn16/OpOut16 |
| `pkg/mir2/vm.go` | PortIO interface, port I/O execution |
| `pkg/mir2/condcall.go` | FoldConditionalCalls pass |
| `pkg/mir2/contracts_pbqp.go` | PBQP-PFCCO solver |
| `pkg/mir2/contracts.go` | OptimizeContractsGreedy exported, PBQP not default |
| `pkg/mir2/z80codegen.go` | OpCallCond/OpIn8/OpOut8/OpIn16/OpOut16 codegen, foldCondCall peephole |
| `pkg/mir2/grace_runner.go` | cond-call-fold Grace rule |
| `pkg/mir2qbe/codegen.go` | defNarrow u8 truncation fix |
| `pkg/pipeline/pipeline.go` | UseVIR removed from default, FoldConditionalCalls wired |
| `pkg/nanz/parse.go` | `loop {}` keyword |
| `cmd/mzv/main.go` | --net/--tls flags, single stdin reader, port wiring |
| `cmd/mzv/port_host.go` | VMPorts (PortIO impl), registerPortHosts |
| `cmd/mzv/net_host.go` | TCP/TLS connect, readPump, control port |
| `cmd/mzv/tui_host.go` | non-blocking tui_read_key |

## Metrics

| What | Count |
|------|-------|
| Commits pushed | ~25 |
| fun/ programs compiling | 29/29 |
| IRC servers tested | 2 (Libera TCP, DarkScience TLS) |
| Port I/O opcodes added | 4 (In8, Out8, In16, Out16) |
| Conditional CALL levels | 3 (peephole, MIR2, Grace) |
| PBQP-PFCCO tests | 14/14 |
| QBE tests | 13/13 |
