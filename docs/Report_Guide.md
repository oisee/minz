# MinZ Report & Documentation Guide

94 reports, 21 docs, 6 READMEs. Here are the ones worth reading.

---

## Architecture & Deep Dives

| Report | What you'll learn |
|--------|------------------|
| [Round-Trip Deep Dive (EN)](../reports/2026-03-10-053-Round_Trip_Deep_Dive_EN.md) | How Nanz code becomes Z80 opcodes — full pipeline walkthrough with hex output verification |
| [MIR2 Architecture](../reports/2026-03-07-029-MIR2_Architecture_And_Progress.md) | SSA-based IR design: block params, Cranelift-style, why not LLVM |
| [Pipeline All Stages](../reports/2026-03-09-035-Pipeline_All_Stages_Walkthrough.md) | Source → Parse → HIR → MIR2 → Alloc → Z80 — every transformation explained |
| [Register Allocator Revolution](../reports/2026-03-10-048-Phase6_Register_Allocator_Revolution.md) | PBQP + contract optimization: how 7 Z80 registers get allocated |
| [Multi-Pass Contracts](../reports/2026-03-13-066-MultiPass_Contracts_Achievement_Article.md) | Bottom-up ABI inference: functions tell callers which registers they want |

## Showcases & Comparisons

| Report | What you'll learn |
|--------|------------------|
| [MinZ C89 vs SDCC](../reports/2026-03-15-081-MinZ_C89_vs_SDCC_Codegen_Comparison.md) | Same C source, MinZ 81B vs SDCC 179B (−55%). Function-by-function analysis |
| [Nanz Z80 Showcase v2](../reports/2026-03-15-084-Nanz_Z80_Showcase_Definitive_v2.md) | 12 verified examples: abs_diff 6B, swap 1B (bare RET), popcount LUT, SMC sprites |
| [Nanz Real ASM Showcase](../reports/2026-03-10-051-Nanz_Real_ASM_Showcase.md) | First showcase with T-state analysis and hand-optimized comparisons |
| [FAT12 MIR2 Comparison](../docs/FAT12_MIR2_Comparison.md) | Nanz vs C89 at MIR2 level — same functions, instruction-by-instruction |

## Frontends & Languages

| Report | What you'll learn |
|--------|------------------|
| [Eight Frontends](../reports/2026-03-15-080-Six_Frontends_Universal_Assert.md) | How 8 languages share one pipeline. Universal assert syntax |
| [PL/M-80 to Z80](../reports/2026-03-09-034-PLM_To_Z80_Pipeline_Walk_Through.md) | Intel's 1970s language compiling through a 2026 IR |
| [ABAP + SQLite + Zork](../reports/2026-03-16-088-ABAP_Frontend_SQLite_Zork.md) | ABAP on Z80, SQLite in MIR2 VM, Zork I running in mze |
| [Lizp Frontend](../reports/2026-03-15-078-Lizp_Frontend_And_Cross_Lang_Import.md) | S-expression frontend, cross-language imports between Nanz/Lanz/Lizp/PL-M |
| [ABAP Wasm Parser](../reports/2026-03-17-089-ABAP_Wasm_QBE_CPM.md) | Embedding a TypeScript parser as Wasm — zero Node.js dependency |

## Features & Milestones

| Report | What you'll learn |
|--------|------------------|
| [Tetris + Language Features](../reports/2026-03-15-074-Tetris_AsmPhase1_PtrCast_ValuePipe.md) | ZX Spectrum Tetris in 853 LOC Nanz. ptr(), value pipes, inline asm |
| [SMC Parameters](../reports/2026-03-11-055-SMC_Parameters_Phase_A_Breakthrough.md) | Self-modifying code: runtime values baked into instruction immediates |
| [Iterator Chain Fusion](../reports/2026-03-02-017-Iterator_Reality_Check.md) | .map().filter().forEach() → single DJNZ loop, zero allocation |
| [Arena Allocator](../reports/2026-03-14-069-Arena_Allocator_Sandbox_Sizeof.md) | Bump allocator with ^Arena pointer receiver, compile-time sizeof |
| [6502 Backend](../reports/2026-03-13-067-6502_Backend_E2E_Harness.md) | MOS 6502 alive: 35 tests, dual-VM oracle, Apple II/C64/BBC Micro |
| [Nanz Language Sprint](../reports/2026-03-15-073-Nanz_Language_Sprint_Six_Features.md) | Enums, type aliases, module imports, three string types, pipe fusion |

## Toolchain

| Report | What you'll learn |
|--------|------------------|
| [MZA Assembler](../reports/2026-02-23-005-MZA_Table_Driven_Encoder_Report.md) | Table-driven Z80 assembler — every opcode from a data table |
| [MZD Disassembler](../reports/2026-02-23-006-MZD_IDA_Analysis_Engine_Report.md) | IDA-like analysis with register tracking and ABI inference |
| [MZE Emulator](../reports/2026-02-23-007-MZE_FUSE_Test_Suite_Report.md) | 1335/1335 FUSE tests — 100% Z80 instruction coverage |
| [MZX Spectrum](../reports/2026-02-23-008-MZX_Spectrum_Emulator_Report.md) | T-state accurate ZX Spectrum emulator with AY sound |
| [ObjC Canvas + Demoscene](../reports/2026-03-16-089-ObjC_Canvas_Demoscene_Multi_Frontend.md) | Cross-language canvas, plasma/diamond/XOR effects, native QBE compilation |
| [VSCode Extension](../reports/2026-03-02-019-VSCode_Tooling_Sprint_Report.md) | LSP, diagnostics, compile-on-save, DeZog debugging |

## Filesystem & Applications

| Report | What you'll learn |
|--------|------------------|
| [FAT12/16 Library](../reports/2026-03-17-090-E2E_MultiChannel_FatFS_Verification.md) | 5-channel cross-verification: Nanz writes, gcc/C89/raw verify |
| [FAT16 Support](../reports/2026-03-18-092-FAT16_Support.md) | FAT12/16 auto-detection, 16MB volumes, multi-cluster files |
| [TUI Framework](../reports/2026-03-17-090-Universal-TUI-Framework.md) | Three-level TUI: flat API → OOP UFCS → compile-time metafunctions |
| [Week In Review](../reports/2026-03-18-094-Week_In_Review_Sprint_0311_0318.md) | 253 commits in one week — the complete summary |

## Bug Analysis & Fixes

| Report | What you'll learn |
|--------|------------------|
| [Overnight Marathon](../reports/2026-03-16-086-Overnight_Marathon_Results.md) | 12-hour session: C99/C11 features, nested structs, function pointers |
| [Codegen Hardening](../reports/2026-03-15-083-Z80_Codegen_Hardening_Session.md) | div/mod, caller-save liveness (−63% PUSH), C89 17/18 OK |
| [BUG-003 Fix](../reports/2026-03-12-062-BUG003_PtrIdx_While_Loop_Fixed.md) | ptr[i] in while loop — root cause analysis and fix |
| [DJNZ + Deprecation](../reports/2026-03-18-093-Deprecation_Const_NC_DJNZ.md) | elimJrToRet eating DJNZ exit RETs, old backends archived |

---

## READMEs

| Location | What it covers |
|----------|---------------|
| [examples/abap/README.md](../examples/abap/README.md) | ABAP on Z80 — setup, examples, architecture, why |
| [examples/objc/README.md](../examples/objc/README.md) | ObjC plasma/shapes/dynamic — function-by-function explanation |
| [stdlib/tui/README.md](../stdlib/tui/README.md) | TUI framework — three levels, all examples |
| [stdlib/fs/fat12.minz](../stdlib/fs/fat12.minz) | FAT12/16 library source (inline docs) |
| [docs/TUI_Framework_Guide.md](TUI_Framework_Guide.md) | Complete TUI guide with metafunctions |
| [docs/Nanz_Language_Book_v5.md](Nanz_Language_Book_v5.md) | 21 chapters + 8 appendices |

---

## ADRs (Architecture Decision Records)

| ADR | Decision |
|-----|----------|
| [ADR-0006](../docs/adr) | Register allocator constraints (blocking NC on CP/M) |
| [ADR-0029](../docs/adr/0029-function-pointer-abi.md) | Function pointer ABI design |
| [ADR-0030](../docs/adr/0030-lir-constraint-architecture.md) | LIR + WFC + PBQP constraint framework |
| [ADR-0033](../docs/adr/0033-lir-pipeline-integration.md) | LIR pipeline integration |
| [ADR-0034](../docs/adr) | Unified constraint framework (ISLE+WFC+PBQP) |
