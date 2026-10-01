# Session Handoff — 2026-03-09

**Branch:** `master` (HEAD: `2e2279b`)
**Working dir:** `/Users/alice/dev/minz-ts` (all clean, nothing uncommitted)
**Tests:** 20/20 packages pass (`go test ./pkg/... -vet=off`)

---

## What was done this session

### 1. `--emit=hir` — structural HIR dump (commit `54612ad`)

Added `--emit=hir` as a proper typed-tree dump, analogous to `--emit=mir2`.

- **New file**: `minzc/pkg/hir/dump.go` — `hir.Module.Dump()` prints typed AST nodes with types on every expression, like `((A:u8) > (B:u8)):bool`
- **Pipeline**: `pkg/pipeline/pipeline.go` — added `Steps.HIR string` field, captured before `hir.LowerModule()`
- **CLI**: `cmd/minzc/main.go` — `--emit=hir` branch added; help text updated

Key distinction: `--emit=nanz` pretty-prints HIR back as Nanz *source syntax*; `--emit=hir` shows the typed Go struct tree. They are different things.

### 2. Full PL/M → Z80 pipeline walkthrough (reports #034, #035)

All emit stages working end-to-end for PL/M input:
```bash
mz file.plm --emit=nanz       # Transpile to Nanz source
mz file.plm --emit=hir        # HIR typed-tree dump
mz file.plm --emit=mir2-raw   # MIR2 before optimisation
mz file.plm --emit=mir2       # MIR2 after DSE + ReorderBlocks
mz file.plm                   # .a80 assembly (default)
mz file.plm -o prog.com -t cpm  # CP/M binary
```

**Bug found in PL/M frontend**: body-local `T` variable in `fib` inferred as `u16` instead of `u8`. Nanz round-trip fixes it (Nanz infers from RHS). Assembly output is byte-identical either way (HIR→MIR2 coerces types), so it's cosmetic only.

### 3. Native PL/M-80 V4.0 compiler comparison (report #036)

Built `plm80c` (Mark Ogden's C port of Intel PL/M-80 V4.0) from source on macOS arm64.

Three macOS compat fixes needed:
1. `#include <limits.h>` in `option.c` (CHAR_BIT undefined)
2. `fcloseall` stub in `shared/os.c` (macOS doesn't have it)
3. Link without `exit.o` (duplicate `_Exit` symbol conflict with `os.o`)

Result: PL/M-80 compiler binary at `/tmp/plm80c`. Test file at `/tmp/report_demo_native.plm`.

**Comparison results** (PL/M-80 V4.0 8080 vs our MIR2 Z80):
| Function | PL/M V4.0 | MIR2 Z80 | Δ |
|----------|-----------|-----------|---|
| ABS_DIFF | 33 bytes  | 12 bytes  | −64% |
| FIB      | 47 bytes  | 31 bytes  | −34% |
| Total    | 80 bytes  | 43 bytes  | −46% |

Root cause: PL/M stores all variables in absolute memory; every access is a load/store round-trip. Our MIR2 keeps loop variables in registers (C/D/E).

See `reports/2026-03-09-036-Native_PLM80_vs_MIR2_Codegen_Comparison.md` for full annotated assembly (8080 mnemonics translated to Z80 syntax for readability).

### 4. feat/mir2 merged to master via PR #14

Branch `feat/mir2` merged. All subsequent work is on `master`.

### 5. ADR-0010 Phase 2 — loop variable register hinting (commit `7234d0a`)

Pre-existing uncommitted work, now committed:

- **`minzc/pkg/optimizer/loop_var_hinter.go`** (new, 193 lines): optimizer pass that scans loop bodies, assigns `RegHintB/HL/DE/A` to heavily-used locals so allocator keeps them in physical registers
- **Pre-coloring** in `generateFunction()`: hinted locals claim their physical register before linear scan, instruction temps can't steal them
- **`Local.Hint` field** in `ir.go`: allows semantic/optimizer to annotate locals with register attractors
- **`FunctionContract`** struct in `ir.go`: typed calling convention (not yet wired into codegen, foundation for Phase 3)
- **IXH/IXL/IYH/IYL** undocumented Z80 register support added to `ir.go`
- **Local addressing fix**: params+locals at `$F100+` (not `$F000+`) to avoid collision with instruction-temp fallback addresses
- **Fake instructions** in `z80asm/fake_instructions.go`: `LD HL,SP` / `LD IX,SP` / `LD IY,SP` pseudo-ops (invalid real Z80, MZA expands them)

---

## PL/M documentation index

Everything is documented — no gaps:

| Doc | What |
|-----|------|
| `docs/adr/0014-plm80-frontend-strategy.md` | ADR: why we built a PL/M-80 frontend and design decisions |
| `reports/2026-03-08-031-PLM80_Parser_Corpus_100pct.md` | Parser: 26/26 Intel 80 Tools files parse (100%) |
| `reports/2026-03-08-032-PLM80_HIR_Coverage_And_Pipeline.md` | HIR lowering: 1338 funcs / 943 globals / 11661 stmts |
| `reports/2026-03-08-033-Nanz_PLM_Transpiler_Plan.md` | Nanz transpiler design |
| `reports/2026-03-09-034-PLM_To_Z80_Pipeline_Walk_Through.md` | First E2E walk-through |
| `reports/2026-03-09-035-Pipeline_All_Stages_Walkthrough.md` | All 7 emit stages with real output; 3-path comparison |
| `reports/2026-03-09-036-Native_PLM80_vs_MIR2_Codegen_Comparison.md` | Native plm80c vs MIR2 Z80 side-by-side |

PL/M frontend source: `minzc/pkg/plm/` — `ast.go`, `lex.go`, `parser.go`, `preprocess.go`, `lower.go`, `corpus_test.go`

---

## Current state / what's next

### Known bottleneck
Iterator chains: ~5x performance overhead because register allocator still puts most virtuals through `$F0xx` memory. The `LoopVarHinter` pass (ADR-0010 Phase 2) is now in place — **next step is to benchmark it** and see how much the pre-coloring moves the needle.

### Suggested next task: benchmark LoopVarHinter
```bash
cd minzc
# Run iterator chain examples, compare T-states before/after
mz ../examples/iterators/forEach.minz --emit=mir2 | grep -A5 "loop"
mze ../examples/iterators/forEach.minz --profile  # T-state profile
```

Baseline from CLAUDE.md: forEach 119T, map+forEach 171T (after ADR-0010 Phase 1 caller-save elimination).
Target: keep loop variables in B/C/D/E/HL instead of $F0xx → expect ~43T per element (from hand-optimized reference).

### Known bugs (pre-existing, not regressions)
- `z80testing`: `TestTSMCCorrectness` etc. fail — shadow register EXX emitted before param loading
- Iterator enumerate/reduce broken on Z80 (B register conflict, A overwrite)
- MZR REPL: `compileModule()` returns empty module, `:run` unimplemented

### ADR-0010 Phase 3 (not started)
Wire `FunctionContract` into codegen: typed parameter passing instead of string-based `CallingConvention`. See `ir.FunctionContract` struct added this session.

---

## Repo state
```
git log --oneline -8
2e2279b chore: remove stale week-roadmap/, rage5.gif; add minzc/testdata/
dfcc2f4 chore: spectrum profiler extras, reports #026/#027/#030, ADR-0010 doc, cleanup
7234d0a feat: ADR-0010 phase 2 — loop variable register hinting + pre-coloring
8656696 docs: translate report #036 PL/M listings from 8080 to Z80 syntax
00b592b docs: native PL/M-80 V4.0 vs MIR2 codegen comparison (report #036)
27ebe0e Merge pull request #14 from oisee/feat/mir2
54612ad feat: --emit=hir typed tree dump + full MIR2 pipeline + E2E PL/M→Z80
f57dd35 fix: PLM→Nanz transpiler fidelity — BASED, arrays, index sugar
```

## Key file paths (quick reference)
- Compiler entry: `minzc/cmd/minzc/main.go`
- PL/M frontend: `minzc/pkg/plm/`
- HIR package: `minzc/pkg/hir/` (`hir.go`, `lower.go`, `dump.go`)
- MIR2 backend: `minzc/pkg/mir2/`
- Z80 codegen (old MIR1): `minzc/pkg/codegen/z80.go`
- Loop var hinter: `minzc/pkg/optimizer/loop_var_hinter.go`
- Pipeline: `minzc/pkg/pipeline/pipeline.go`
- Tests: `go test ./pkg/... -vet=off` from `minzc/`
- Binary: `minzc/mz` (aliased locally; run from project root for stdlib resolution)
