# Next Session Briefing: VIR Polish + Grace Expansion + Paper Finalization

## Context

VIR solver hit 520/520 = 100% corpus coverage on 2026-03-23. Zero PBQP fallback. The solver is production-quality for leaf and multi-block functions. Three known issues remain.

## Priority 1: Fix abs_diff CFG Solver

The CFG-aware solver returns unsat for `abs_diff`, falling back to per-block mode (13 insts vs optimal ~9). The root cause is likely interference constraints being too tight when conditional paths share a vreg with different liveness.

**Where to look:**
- `minzc/pkg/vir/cfgsolver.go` — `SolveCFG` generates per-block variables with edge constraints
- The two conditional paths (a>b and a<=b) have different register needs
- Check if interference constraints from then-path leak into else-path variables

**Test:** `go test ./pkg/vir/ -run "TestVIR_vs_SDCC" -v` — abs_diff should show "CFG solver" not "falling back to per-block"

## Priority 2: Fix Non-Deterministic Coalescing

`coalesceVRegs` in solver.go iterates over a Go map, causing non-deterministic merge order. AbsDiff test is flaky.

**Fix:** Sort vreg keys before iterating. Simple, mechanical change.

**Where:** `minzc/pkg/vir/solver.go`, function `coalesceVRegs`

## Priority 3: Grace on PIR Expansion

Current Grace pass has 3 rules:
1. Dead register before RET
2. EX DE,HL when target dead
3. ADD A, 0 removal

**Potential new rules:**
- **DEC HL elimination** — When caller doesn't read HL after call, callee's DEC HL before RET is dead. Classic `st_word` optimization.
- **LD r,r self-move** — Already in peephole, but Grace could catch cross-block patterns
- **Fallthrough optimization** — Reorder blocks to maximize JR-to-next-instruction (removes JP)
- **Conditional return chaining** — `JR cc, .skip / RET / .skip: LD A, r / RET` → `RET cc / LD A, r / RET`

**Where:** `minzc/pkg/vir/pipeline.go`, function `gracePass`

## Priority 4: Paper Draft Update

`research/abi-paper/vir-solver-draft.md` needs:
- Section 4 (Evaluation) updated to 520/520
- New subsection: "Handling Inline Assembly" (OpAsmBlock approach)
- New subsection: "Inline Runtime Expansion" (per-call-site inlining)
- Abstract updated (was 447 functions, now 520)

## Priority 5: VIR as CLI Default

Currently the CLI (`cmd/minzc/main.go`) defaults to `--lir=true`, which uses the LIR backend. VIR is only default for programmatic use (`DefaultOptions()`). Consider:
- Add `--vir` flag to CLI
- Or: make VIR the default in CLI too (risky — LIR handles more edge cases for non-corpus programs)
- Safest: `--vir` flag that overrides `--lir`, with a fallback chain: VIR → LIR → PBQP

**Where:** `minzc/cmd/minzc/main.go` line 208

## Key Files

| File | Purpose |
|------|---------|
| `minzc/pkg/vir/solver.go` | Z3 encoding, pre-solver passes, model parsing (~1750 LOC) |
| `minzc/pkg/vir/cfgsolver.go` | CFG-aware multi-block encoding (~250 LOC) |
| `minzc/pkg/vir/pipeline.go` | Orchestration, peephole, Grace, inline runtime (~1100 LOC) |
| `minzc/pkg/vir/bridge.go` | MIR2 → VIR translation (~560 LOC) |
| `minzc/pkg/vir/isle.go` | ISLE combining, load16_le/store16_le fusion (~380 LOC) |
| `minzc/pkg/vir/z80.go` | Z80 machine descriptor, 71+ patterns (~470 LOC) |
| `minzc/pkg/vir/assert_test.go` | 55 Z80-verified asserts |
| `minzc/pkg/vir/corpus_test.go` | 216/216 Nanz corpus |
| `minzc/pkg/vir/corpus_c89_test.go` | 304/304 C89 corpus |

## Quick Verification

```bash
cd minzc
go test ./pkg/vir/ -run "Assert" -v       # 55 asserts, ~0.3s
go test ./pkg/vir/ -run "NanzCorpus" -v    # 216/216, ~20s
go test ./pkg/vir/ -run "C89" -v           # 304/304, ~10s
go test ./pkg/vir/ -run "Compare" -v       # VIR vs SDCC, ~0.5s
```
