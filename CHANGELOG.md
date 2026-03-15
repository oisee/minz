# Changelog

Development log for the MinZ compiler project. Each entry links to the full report.

For pre-MIR2 history (v0.1–v0.15, 2025), see git log.

---

## 2026-03-15

**42 commits. 8 reports (#073–#080). [Full context →](reports/2026-03-15_u7_context.md)**

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
