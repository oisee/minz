# Changelog

Development log for the MinZ compiler project. Each entry links to the full report.

For pre-MIR2 history (v0.1–v0.15, 2025), see git log.

---

## 2026-03-26 (Sessions 7-8)

**Birthday marathon continues. Research + language features.**

### @error — Z80-Native Error Propagation
- **`@error(N)`** → `SCF / LD A, N / RET` — set carry flag, error code in A, return (2 bytes, 15T)
- **`@propagate`** → `RET C` — **1 byte** conditional return on carry. Z80's native error propagation.
- **`@check`** → `JR NC, .ok / RET / .ok:` — check + propagate inline
- Pure metafunction expansion — zero parser/AST/semantic changes. 55 LOC.
- Convention: `?` in function name = fallible (enforcement TBD)
- [Codegen Report](docs/Error_Propagation_Codegen.md) | [Design Doc](docs/Error_Propagation_Design.md) | [Example](examples/nanz/14_error_propagation.nanz)

### GPU Exhaustive Table — Complete ≤5v
- **≤4v:** 156,506 shapes, 40 sec, COMPLETE
- **≤5v:** 17,366,874 shapes, 20 min, COMPLETE
- **6v sample:** 537K shapes (45 min partial run)
- **Feasibility cliff:** 6v = only 0.9% feasible (99.1% impossible!)
- **Composition verified:** 13.2M shapes, 5.06T avg overhead, 480 cases composition BEATS GPU
- **Treewidth:** 99.5% random graphs classically tractable, 46.3% for dense corpus

### Frontend Sprint
- **PL/M:** 5 examples created (hello, assert, showcase, abs_diff_fib, sum_array) — all compile
- **Lizp:** all 8 examples compile + assemble (MZA fixes resolved all gaps)
- **Pascal:** SwitchStmt/VarDeclStmt traversal fix. Stdlib blocked on InlineTrivial (VIR P6)

### Research
- 4 papers with companion files + data
- Cross-compiler analysis: Nanz `ADD A, A` = SDCC `ADD A, A` (ISA-intrinsic proof)
- SDCC comparison: swap 20:0, minmax 63:11, abs_diff 13:4
- C89 signatures: 292 funcs → 129 unique (55.8% reuse)
- Paper C: compositional regalloc from solved sub-shapes
- Anytime-optimal 5-level pipeline doc (263 lines with ASCII diagrams)

---

## 2026-03-25 (Sessions 3-4)

**~25 commits across 3 repos. 3 ZX Spectrum screenshots. Paper A draft.**

### SQL on Z80 CP/M
- **ZSQL.COM** — Interactive SQLite REPL on CP/M. `CREATE TABLE OK`, `INSERT OK`, `SELECT` returns row data via I/O port bridge. Readline with BDOS 0x0A, prompt loop, .quit/.help commands. [zsql.nanz →](examples/nanz/zsql.nanz)
- **SELECT returns data** — `sqlite_query` → `sqlite_step` → `sqlite_column_text` → row output. Asm wrappers bypass Z3 register pressure (hardcoded `EX DE,HL / LD HL,(db) / CALL sqlite_query`).

### ABAP on ZX Spectrum
- **ZSQL MARA session** — CREATE TABLE mara → INSERT 4 materials → SELECT WHERE mtart=FERT → filtered results. Enterprise SQL on 1982 hardware. [Screenshot →](media/zsql_mara_zx_spectrum.png)
- **ALV grid** — Material Master with 5 rows (MATNR/MTART/MEINS/MAKTX) + ABAP source code visible below. [Screenshot →](media/mara_alv_zx_spectrum.png)
- **ZSQL session** — CREATE TABLE, INSERT, SELECT with pipe-separated columns. [Screenshot →](media/zsql_zx_spectrum.png)
- **Embedded ZX font** — 96-char charset (768 bytes, ASCII 32-127 including lowercase). Target-aware ABAP runtime: `--target=spectrum` swaps BDOS for `_zx_putchar` + console port $23 input.

### MZA Assembler Fixes
- **`pass >= 2`** — Multi-pass data emission. DB/DW/DS directives were silently dropped on pass 3+ (JRS expansion caused reconvergence). Binary went from 923→1465 bytes. Root cause of VIR blank output.
- **LD IXH,H fake instruction** — DD prefix conflict expansion: `LD IXH,H` → `LD A,H / LD IXH,A`. Covers all 8 IXH/IXL/IYH/IYL × H/L combinations.

### VIR Backend Fixes (from minz-vir)
- **Grace INC dedup** — `INC HL / INC HL / RET` → Grace removed second INC ("dead before RET"). Fix: preserve INC/DEC on return-capable registers.
- **Tail call guard** — `CALL+RET→JP` restricted to local labels. External calls keep CALL+RET. Fixes parameter passing for Z3-compiled functions.
- **Post-emit validation** — Catches Z3 16-bit load clobber pattern (`LD HL,(addr) → LD L,r` overwrite). Falls back to PBQP.
- **String pool ordering** — `spliceVIRFallback` + all-Z3 path: globals before strings. Prevents MZA trailing-zero trim from clipping string data.
- **DD prefix routing** — `fixDDPrefixConflict` in all emit paths. Routes H/L↔IXH/IXL through A.
- **OpAsmBlock CFG solver** — CFG solver preserves AsmTemplate. Inline asm functions get proper bodies.

### GPU Regalloc Table (from z80-optimizer)
- **11.6M feasible 5-vreg entries** (17.2M patterns, 20 min dual GPU)
- **299K feasible 6-vreg entries** (corpus-derived)
- **1325 real VIR corpus entries** (639 feasible, 48.2%)
- **Width-aware kernel** — 16-bit vregs restricted to BC/DE/HL/mem
- **Direct PIR emit** — Table hit → pattern select → asm. Zero solver. O(1) compilation.
- **315 unique signatures** — 97.8% convergence. The table is effectively complete.

### Research
- **Phase transition** — Cliff at 16 register locations. Z80(15)=81% coverage, 6502(3)=100%.
- **88.2% cross-frontend transfer** — Train Nanz+PL/M, test ABAP+Screen+SQLite.
- **Paper A draft** — *"Register Allocation as a Solved Game: Exhaustive GPU Tables for Sub-Cliff Architectures"*. 7 sections, full evaluation, honest caveats. [ADR-0040](docs/adr/0040-island-of-optimality-regalloc.md)

---

## 2026-03-24 (Session 2)

**4 commits. 1 report (#113). [Full context →](reports/2026-03-24-113-CPM-Screen-Polish-SQLite-Bridge.md)**

- **[CP/M Screen Polish + Z80 SQLite Bridge](reports/2026-03-24-113-CPM-Screen-Polish-SQLite-Bridge.md)** — One session from truncated labels to SELECT on Z80. Inline Z80 asm for BDOS-safe TUI primitives (puts_padded, tui_clear, tui_reset). ALV table with 5 data rows on CP/M. Interactive field editing (TAB/Enter). Z80 SQLite bridge via I/O ports $41/$43/$45/$47 — `sqlite_column_text(1, 0) → "Alice"`.
- **stdlib/sql/sqlite.nanz** — New stdlib module: sqlite_open/exec/query/step/column_text/column_int/finalize/close. Z80 asm bodies for I/O port protocol, mzv host functions (pre-existing).
- **Cross-session coordination** — Used dedelulu send/explore to validate Z3 solver handles all regalloc workarounds natively (VIR: 27/27 functions, correct while loops).

---

## 2026-03-15

**42 commits. 8 reports (#073–#080). [Full context →](contexts/2026-03-15_u7_context.md)**

- **[Five Frontends, Universal Assert](reports/2026-03-15-080-Five_Frontends_Universal_Assert.md)** — Nanz, Lanz, Lizp, PL/M-80, Pascal all compile through one HIR → MIR2 → Z80 pipeline. Compile-time assert in all 5 frontends (45/45 verified). Pascal → CP/M hello world.
- **[Nanz Language Book v5.3](docs/Nanz_Language_Book_v5.md)** — 21 chapters + 8 appendices (3195 lines). Five-frontend architecture, universal assert syntax, cross-language imports, transpilation. [PDF](releases/v0.21.4/Nanz_Language_Book_v5.pdf) / [EPUB](releases/v0.21.4/Nanz_Language_Book_v5.epub).
- **[ZX Spectrum Tetris](examples/zx/tetris.nanz)** — 853 LOC Nanz → 2176 lines Z80 asm (48 functions). SRS wall kicks, hold/next/ghost piece, T-spin scoring, attribute-based rendering.
- **[Tetris + Asm Phase 1 + ptr() + Value Pipe](reports/2026-03-15-074-Tetris_AsmPhase1_PtrCast_ValuePipe.md)** — `ptr(addr)^` peek/poke, `|>` value pipe with constant folding, `asm (ret A) (clob A,F)` precise clobbers, auto-clobber analysis.
- **[Nanz Language Sprint](reports/2026-03-15-073-Nanz_Language_Sprint_Six_Features.md)** — enums, type aliases, module imports (4 styles), three string types, pipe/trans named pipelines.
- **[Lizp Frontend](reports/2026-03-15-078-Lizp_Frontend_And_Cross_Lang_Import.md)** — Scheme/Lisp dialect: `defmacro`, threading (`->`, `->>`), `cond`/`when`/`unless`. Desugars to Lanz.
- **[Pascal Frontend](reports/2026-03-15-079-Pascal_Frontend_Corpus_And_Feature_Matrix.md)** — TP3.0 subset → HIR → Z80. Records, arrays, `for`/`while`/`repeat`/`case`. `WriteLn` → BDOS via inline asm.
- **Lanz Frontend** — S-expression IR, 1:1 HIR mapping, round-trips via `--emit=lanz`. Used internally by `@derive_*`.
- **Cross-language imports** — `import mathlib { double }` resolves `.nanz`/`.lanz`/`.lizp`/`.plm`/`.pas` automatically. Circular detection across boundaries.
- **MetaRuntime** — compile-time introspection + emit via Lanz + MIR2 VM. Native Go metafunctions.
- **MZD** — recursive descent analysis now default mode.
- **Bug fixes** — `defextern` hex desugaring, Pascal BDOS registers, array index load, `OpAddrOf` 8-bit guard, `InlineTrivial` ptr-op exclusion, `ADD HL,IX` guard. BUG-009 through BUG-014 documented.

## 2026-03-13

- **[Nanz Language Book v4](docs/Nanz_Language_Book_v4.md)** — 14 chapters + 4 appendices. New: `as` cast, signed comparison, PreallocCoalesce, trivial inliner, 6502 backend, `mzn` native compiler, `@smc`. Available as [PDF](releases/v0.21.2/Nanz_Language_Book_v4.pdf) / [EPUB](releases/v0.21.2/Nanz_Language_Book_v4.epub).
- **[MinZ vs Nanz Feature Gap Analysis](docs/MinZ_vs_Nanz_Feature_Gap.md)** — Complete gap analysis: enums, @error, imports, strings, type aliases. Implementation proposals with effort estimates.
- **`mzn` native compiler** — `mzn file.nanz` compiles to AMD64 via C99 or QBE. VSCode integration. `expr as type` cast syntax.
- **[Showcase #068: PreallocCoalesce Impact](reports/2026-03-13-068-Showcase_ForEachEdge_SignedCmp_PreallocImpact.md)** — 6 assembly files improved: `mapInPlace` 5 instructions → 1 DJNZ; `factorial_fold` mul16 eliminated; `forEach` trampoline removed. ForEachEdge refactor (~75 LOC removed). Signed i8/i16 comparison working in VM.
- **[BUG-001 Phase 1: PreallocCoalesce](reports/2026-03-13-067-BUG001_PreallocCoalesce_Wired.md)** — Pre-allocation coalescing wired into pipeline.
- **[MOS 6502 Backend: E2E Harness](reports/2026-03-13-067-6502_Backend_E2E_Harness.md)** — 35/35 tests. Dual-VM oracle (MIR2 VM vs sim6502). Console I/O for Apple II/C64/BBC Micro.
- **[Phase 6f: Trivial Inliner](reports/2026-03-13-066-MultiPass_Contracts_Achievement_Article.md)** — `swap(a,b).1 == a` → zero instructions. `min_of(a,b)` → `EQU minmax` (0 bytes).
- **[Phase 6e: Multi-Pass Contracts](reports/2026-03-13-065-Phase6e_Complete_MultiPass_Mul16_DJNZ.md)** — mul16 rhs→DE nudge, DJNZ counter→B nudge, BC★/DE★ LUT codegen.
- **[Showcase #064](reports/2026-03-13-064-Showcase_Update_Phase6e_BCstar_And_BugFixes.md)** — BUG-002/004/005 fixed. 23/23 showcase.

## 2026-03-12

- **[BUG-003 Fixed: `ptr[i]` in While Loop](reports/2026-03-12-062-BUG003_PtrIdx_While_Loop_Fixed.md)** — 5 interacting codegen bugs. Real pointer-loop programs now work.
- **[All 23 Showcase Passing](reports/2026-03-12-061-All_Showcase_Passing_ForRange_And_Codegen_Fixes.md)** — ForRange precomputation, u16 global loads, constant store fix, EX DE,HL swap semantics.
- **[Codegen Fixes & Allocator Roadmap](reports/2026-03-12-060-Codegen_Fixes_And_Allocator_Roadmap.md)** — u8→u16 zero-extension, self-pointer load, mul16 operand order. +3 showcase.

## 2026-03-11

- **[Frontend & Backend Diagnostic](reports/2026-03-11-059-Frontend_Backend_Diagnostic.md)** — Full system health: 25/28 green, 160+ MIR2 tests.
- **[`range(lo..hi)` + Parallel Copy Fix](reports/2026-03-11-057-Range_Iterator_And_Parallel_Copy_Fix.md)** — Counter-based iterator: `fold` → 1 instruction/iteration.
- **[Nanz Z80 Showcase Definitive](reports/2026-03-11-056-Nanz_Z80_Showcase_Definitive.md)** — 15 verified examples with T-state analysis.
- **[`@smc` Parameters Phase A](reports/2026-03-11-055-SMC_Parameters_Phase_A_Breakthrough.md)** — Baked immediates: `@smc r0: u16` → `LD HL, imm16`. Compiled sprites 36T/row.
- **[Output Quality & Allocator Trilogy](reports/2026-03-11-054-Nanz_Z80_Output_Quality_And_Allocator_Trilogy.md)** — Struct literals, `LD (HL), n` folding, StorageClass, ADR-0019.

## 2026-03-10

- **[ASM Showcase: `abs_diff` Optimal](reports/2026-03-10-051-Nanz_Real_ASM_Showcase.md)** — 5-pass optimization: 8 → 4 instructions (`SUB C / RET NC / NEG / RET`).
- **[LUT Pointer Selection](reports/2026-03-10-049-LUT_Pointer_Selection_And_PBQP_Edge_Costs.md)** — 21T → 18T LUT. BC★ 14T.
- **[Phase 6: PBQP + IX/IY + Coalescing](reports/2026-03-10-048-Phase6_Register_Allocator_Revolution.md)** — Greedy → PBQP. 4 simultaneous pointers, zero spills.

## 2026-03-09

- **[Nanz Week 1: UFCS + Interfaces](reports/2026-03-09-046-Nanz_Week1_RCA_And_Phase6_Plan.md)** — Go-style interfaces → direct `CALL`. Zero vtable, zero indirection.
- **[MIR2→QBE Native Backend](reports/2026-03-09-045-MIR2_To_QBE_Native_Backend_And_Correctness_Oracle.md)** — Correctness oracle: 4/4 E2E tests.
- **[E2E Overview & Roadmap](reports/2026-03-09-043-E2E_Overview_Architecture_And_Roadmap.md)** — Architecture deep-dive.
- **[LUTGen](reports/2026-03-09-038-Nanz_PLM_HIR_MIR2_Z80_E2E_Snapshot.md)** — `u8<0..255>` → compile-time table.
- **[Native PL/M-80 vs MIR2](reports/2026-03-09-036-Native_PLM80_vs_MIR2_Codegen_Comparison.md)** — −46% code size vs Intel PL/M-80 V4.0.

## 2026-03-08

- **[PL/M-80 Frontend: 26/26 corpus](reports/2026-03-08-032-PLM80_HIR_Coverage_And_Pipeline.md)** — 100% Intel 80 Tools; 1338 functions → HIR → MIR2 → Z80.

## 2026-03-07

- **[MIR2 Architecture](reports/2026-03-07-029-MIR2_Architecture_And_Progress.md)** — PBQP domain map.
- **[MIR2 Codegen Quality Sprint](reports/2026-03-07-028-MIR2_Codegen_Quality_Sprint.md)** — 42 tests, 9 verified functions, DSE.

## 2026-03-04 and earlier

- **[Honest Assessment](reports/2026-03-04-025-Honest_Assessment_Code_Verified.md)** — Every claim verified by live test runs.
- **[MIR Backend Test Suite](reports/2026-03-04-023-MIR_Backend_Test_Suite.md)** — 11 .mir programs, pipeline validation.
- **[VSCode: Edit, Compile & Run](reports/2026-03-04-022-VSCode_Tooling_And_Codegen_Fixes.md)** — Cmd+Alt+R compiles and runs.
- **[VSCode Tooling Sprint](reports/2026-03-02-019-VSCode_Tooling_Sprint_Report.md)** — LSP, syntax highlighting, DeZog.
- **[Register Allocator Overhaul](reports/2026-03-02-018-Register_Allocator_Overhaul_Results.md)** — 7.8x iterator speedup.
- **[Iterator Status](docs/Iterator_Implementation_Status.md)** — 11/11 E2E correct.
- **[Project Status](reports/2026-03-01-015-Project_Status_And_Next_Steps.md)** — v0.19 roadmap.

---

## Pre-MIR2 era (2025)

| Version | Date | Highlights |
|---------|------|------------|
| v0.15.0 | 2025-08-24 | Array literal optimization, Ruby string interpolation |
| v0.14.0 | 2025-08-13 | ANTLR parser (replaced tree-sitter), 75% compile rate |
| v0.11.0 | 2025-08-11 | Cast interface system, zero-cost dispatch |
| v0.9.6 | 2025-08-05 | Function overloading, UFCS |
| v0.8.0 | 2025-07-30 | TRUE SMC lambda, 14.4% fewer instructions |
| v0.7.0 | 2025-07-28 | TSMC reference system, diagnostics |
| v0.5.0 | 2025-07-15 | Self-modifying code framework |
| v0.1.0 | 2025-06-20 | Initial implementation |
