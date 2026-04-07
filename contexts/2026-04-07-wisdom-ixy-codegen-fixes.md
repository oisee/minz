# Session Wisdom — 2026-04-07 — IXY Codegen Fixes & Tetris Diagnosis

## What Was Done

### Fixes (pushed to master)

1. **`a0f1094c` — LD (HL),IXL fix** in `minzc/pkg/mir2/z80codegen.go`
   - Root cause: `isPairReg("IX")` matched before `isIXYReg` guard in OpStore path.
   - `lowByte("IX")` = `"IXL"` → invalid `LD (HL), IXL`.
   - Fixed 3 sites: excluded IX/IY from `isPairReg` branch, let them fall through to route-through-A.
   - Also fixed HL-chain store path (global field stores).

2. **`ab362576` — BIT/SET/RES IXY rejection** in `minzc/pkg/mir2/z80codegen.go`
   - Root cause: `emitBitRegOp` had wrong comment claiming IXH/IXL "first-class" for BIT/SET/RES. CB prefix encoding has no DD/FD variant.
   - Single-point fix: added `isIXYReg(reg)` rejection in `emitBitRegOp` (line 1986). All 6 callers auto-fallback to AND/OR mask.
   - Also guarded direct BIT emit at line 5667.
   - Updated 6 tests that asserted the invalid behavior.

3. **VIR build fix** — added stub fields `MaxVregs`, `NLocSets8`, `NLocSets16` to `EnrichedBinaryTable` and `EnrichedIndexOfWithLocSets` wrapper to unblock VIR package compilation.

### Reports (pushed to master)

- `reports/2026-04-06-Claude-FatFS-Precision-Blockers.md` — 4 FatFS blockers
- `reports/2026-04-06-Claude-ObjC-Snippet-Editorial-Review.md` — ObjC simple.a80 audit
- `reports/2026-04-07-Sprint-Session-Report-RU.md` — Russian sprint summary
- `reports/2026-04-07-Next-Sprint-Plan-RU.md` — Russian sprint plan with seed refs
- `reports/2026-04-07-Claude-IXY-Half-Opcode-Audit.md` — Full IXY opcode family audit
- `reports/2026-04-07-Tetris-Runtime-Verification.md` — No regression from IXY fixes
- `reports/2026-04-07-Claude-Tetris-Probe-Cutpoints.md` — 3-probe isolation plan
- `reports/2026-04-07-Claude-PIECE-DX-Address-Audit.md` — Global array address OK, pointer loop broken
- `reports/2026-04-07-Claude-Loop-Header-Param-Repro.md` — Standalone repro + PBQP comparison
- `reports/2026-04-07-Claude-Loop-Header-Param-Fix.md` — Updated by codex with actual 3-bug fix

### Diagnosis (not fixed in minz — fix is in minz-vir)

- **Tetris immediate game_over**: traced to `piece_dx`/`piece_dy` returning garbage because VIR CFG solver drops edge moves for loop-header block args.
- Three independent VIR bugs identified by codex (see updated report):
  1. `EliminateDeadConsts` removes terminator-arg vregs (const 0 for counter init)
  2. `paramUsedLater` doesn't scan terminator args → param not pinned at ABI register
  3. `emitFromTable` bypasses edge moves for multi-block functions

## What Was NOT Done

- **SRL/SRA/SLA on IX halves** — identified as latent same-class bug, parked per codex instructions.
- **VIR edge-move fix** — belongs in minz-vir repo. Codex applied the fix there.
- **`&local_var` FatFS blocker** — identified but not started. Next in priority queue.
- **u8 truncation in QBE** — identified, not started.

## Gotchas & Non-Obvious Decisions

1. **`isPairReg` ordering matters**: IX/IY are both "pair regs" AND contain IXY half-regs. Any `isPairReg(val)` branch that calls `lowByte(val)` MUST exclude IX/IY first, otherwise `lowByte("IX")="IXL"` leaks into instructions that don't support it.

2. **Z80 CB prefix rule**: BIT/SET/RES/RLC/RRC/RL/RR/SLA/SRA/SRL — ALL use CB prefix. None support IXH/IXL/IYH/IYL as register operands. Only `{A,B,C,D,E,H,L}` valid. The `(IX+d)` forms use DDCB prefix with displacement, which is a different encoding — NOT the same as "IXL as a register".

3. **`isSimpleReg` is too permissive**: returns true for any alphabetic string including "IXL", "IYH". Not a good guard for instruction-validity checks.

4. **VIR vs PBQP paths diverge**: same function can produce correct code via PBQP and broken code via VIR. The test `TestDifferential_Z80_vs_SDCC` uses `pipeline.CompileHIR` which goes through VIR+PBQP-fallback, while `minzc` CLI also uses VIR. Direct PBQP-only path requires custom Go script.

5. **Pre-existing `TestStrLenZ80` failure**: checks for decimal byte values in DB, but codegen emits `DB "Hello", 0` (string form). Unrelated to any IXY work.

6. **dedelulu session IDs change**: codex minz-vir went from `hz8hkfc8` to `gv_wsvdk` mid-session. Always `dedelulu explore` before sending.

## Architectural Insights

- The MIR2 Z80 codegen has ~15 different emit sites for `LD (ptr), val` and `LD val, (ptr)`. Each needs independent IX/IY guards. A centralized `emitIndirectStore(ptr, val)` helper would prevent future regressions.
- The VIR CFG solver's edge-move mechanism is fundamentally sound but has gaps around block-argument-only vregs (constants and params that never appear in VIR op Src fields). The fix pattern: scan terminator args in addition to VIR op Src.
- Z3 non-determinism means the same function can get different register assignments across runs. Tests must verify correctness properties (no invalid opcodes, correct semantics) rather than specific register choices.

## Key Artifacts

| Artifact | Path | Description |
|----------|------|-------------|
| IXY LD fix | `minzc/pkg/mir2/z80codegen.go` | 3 sites in OpStore, isPairReg guard |
| IXY BIT fix | `minzc/pkg/mir2/z80codegen.go` | emitBitRegOp + direct BIT guard |
| IXY BIT tests | `minzc/pkg/mir2/z80codegen_test.go` | 6 tests updated to assert AND/OR fallback |
| VIR build stubs | `minzc/pkg/vir/enriched_reader.go`, `enriched_index.go` | Forward-compat fields |
| Repro file | `/tmp/test_lookup_assert.nanz` | Minimal counted-loop pointer-walk |
| FatFS blockers | `reports/2026-04-06-Claude-FatFS-Precision-Blockers.md` | 4 blockers with repros |
| IXY audit | `reports/2026-04-07-Claude-IXY-Half-Opcode-Audit.md` | Full opcode family map |
| Tetris probes | `reports/2026-04-07-Claude-Tetris-Probe-Cutpoints.md` | 3-step isolation plan |
| VIR fix (by codex) | `reports/2026-04-07-Claude-Loop-Header-Param-Fix.md` | 3-bug fix in minz-vir |
