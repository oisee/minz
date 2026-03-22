# Session 2: eZ80 Native Codegen + TUI Library
**Date:** 2026-03-20 (afternoon)
**Branch:** `feat/ez80-backend`

---

## Commits This Session (7 total)

| Hash | Description |
|------|-------------|
| `a4c2af3d` | Phase 1a+1b: TargetInfo, pkg/ez80/ skeleton, DW24/DL directives |
| `2ca04ed8` | Phase 1c: Full pipeline MinZ -> eZ80 assembly -> binary |
| `3664e87a` | ADR-0037: Target-neutral optimization split |
| `2b0aaa2a` | Phase 2: EZ80CostTable for PBQP |
| `ba8d8cf9` | Phase 3: Native MIR2 codegen (replaces wrapper) |
| merge | Merged master (TUI Tetris, Z80-VALIDATE, width-mismatch fix) |
| `c2d08319` | Agon TUI library + hello example + codegen fixes |

## Architecture Established

Three-layer split (ADR-0037):
- **Layer 1:** Target-neutral MIR2 passes (all shared)
- **Layer 2:** Target-parameterized allocation (CostTable interface)
- **Layer 3:** Target-specific codegen (pkg/ez80/)

## Key Files Created

| File | LOC | Purpose |
|------|-----|---------|
| `pkg/codegen/target_info.go` | ~55 | TargetInfo struct (Z80/eZ80/CPM) |
| `pkg/ez80/codegen.go` | ~475 | EZ80Backend + IR-based generator |
| `pkg/ez80/codegen_test.go` | ~350 | 10 tests (backend, codegen, E2E) |
| `pkg/ez80/cost.go` | ~340 | EZ80CostTable (PBQP costs) |
| `pkg/ez80/cost_test.go` | ~140 | 9 cost table tests |
| `pkg/ez80/mir2codegen.go` | ~775 | Native MIR2->eZ80 codegen |
| `stdlib/agon/tui_render.nanz` | ~210 | MOS-based TUI primitives |
| `examples/agon/tui_hello.nanz` | ~22 | TUI hello world for Agon |
| `docs/adr/0036-*.md` | 324 | eZ80 independent backend ADR |
| `docs/adr/0037-*.md` | 332 | Target-neutral optimization split ADR |

## MZA Changes
- `.ASSUME ADL=1` directive (switches to eZ80 ADL mode)
- `DW24` / `DL` directives (3-byte data, agon-ez80asm compatible)
- `DW` stays 2 bytes in all modes

## Corpus Results (89 HIR pipeline files)
- **Codegen:** 89/93 pass (95.7%) — 4 failures are pre-existing (not eZ80)
- **Assembly:** 44/89 pass (49.4%) — remaining are cross-module label prefix mismatch + instruction format bugs
- Parallel session fixing Z80 instruction format bugs (LD A,HL etc.)

## eZ80 Codegen Coverage (MIR2 ops)
Implemented: OpConst, OpMove, OpAdd, OpSub, OpMul (MLT!), OpAnd, OpOr, OpXor, OpNeg, OpNot, OpExt, OpCmp, OpCall, OpCallIndirect, OpLoad, OpStore, OpAddrOf, OpField (LEA!), OpPtrAdd, OpPtrBump, OpAlloca
Terminators: TermRet, TermJmp, TermBrIf, TermDJNZ, TermUnreachable

## Known Issues
1. Cross-module label mismatch: `CALL tui_clear` vs label `agon_tui_render_tui_clear`
2. Some width-mismatch paths still leak (LD A, HL residual)
3. `ADD BC, DE` — only `ADD HL, rr` exists, needs route through HL
4. No TermCondRet support yet
5. No 16/24-bit multiply (__mul16 runtime not implemented)

## What's Next
- Fix cross-module label prefix in eZ80 codegen
- Test on Fab Agon Emulator (agon-cli-emulator)
- Phase 4: LIR MachineDesc for eZ80
- ISLE eZ80-specific combining rules (MLT fusion, LEA fusion)
