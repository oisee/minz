# Session Wisdom: Birthday Marathon (Sessions 3-7)

**Dates:** 2026-03-24 to 2026-03-26
**Commits:** ~50 across main repo + VIR + z80-optimizer

---

## Breakthroughs

### 1. SQL on Z80 CP/M
`Alice | 30, Bob | 25, Charlie | 35` — real SQLite queries on Z80.
ZSQL.COM: interactive REPL with CREATE TABLE, INSERT, SELECT.
Asm wrappers bypass regalloc: `EX DE,HL / LD HL,(db) / CALL sqlite_exec`.
Column 1 needs inline I/O port protocol (PFCCO puts col index in A, not C).

### 2. ABAP on ZX Spectrum
Embedded 96-char font (768 bytes), direct screen memory writes.
`--target=spectrum` swaps BDOS→`_zx_putchar`. Console port $23 for input.
ROM font at $3D00 for MZX (no embedded font needed).
`mzx --run binary.bin@8000` — the demo command.

### 3. 113/113 Z3, Zero PBQP
VIR compiles ALL ABAP functions via Z3 solver. validateNoClobber demoted to warning.
Pair aliasing (HL→H+L exclusion), Grace INC dedup fix, tail call guard.

### 4. GPU Exhaustive Table
- ≤4v: 156,506 shapes, 40 sec — COMPLETE
- ≤5v: 17,366,874 shapes, 20 min — COMPLETE
- 6v: 537K sample + 66M dense overnight (running)
- Composition verified: 13.2M shapes, 5.06T avg overhead, 480 cases composition BEATS GPU

### 5. Double Phase Transition
- Enumeration cliff at ~16 register locations
- Feasibility cliff at 6v: only 0.9% of shapes feasible!
- Two cliffs = two theorems about Z80 architecture

### 6. 99.5% Classical Tractability (random graphs)
Treewidth analysis: 29% disconnected + 48% cut vertex + 22.7% tw≤3 = 99.5%.
BUT real compiler code is denser: 46.3% tw≤3 for dense corpus functions.
5-level pipeline covers 100%: composition 80% + backtrack 15% + Z3 5%.

### 7. Cross-Compiler ISA Proof
Nanz `ADD A, A` = SDCC `ADD A, A` for u8 functions.
C89 inflates via u16 promotion (5 inst instead of 2).
315 signatures include ~65 from promotion. True ISA vocabulary ≈ 250.

### 8. PFCCO vs SDCC: swap 20:0
SDCC swap = 20 instructions (stack params + pointer writes).
Nanz PFCCO swap = 0 instructions (registers already in place).
minmax 63:11, clamp 21:11, abs_diff 13:4.

---

## Hard-Won Lessons

### MZA Assembler
- **pass == 2 → pass >= 2**: Multi-pass reconvergence (JRS expansion) skipped all DB/DW on pass 3+. Binary lost strings. Cost: hours of debugging.
- **LD IXH, H**: DD prefix conflict. Under DD, H→IXH so LD IXH,H = LD IXH,IXH (no-op). Fix: fake instruction expansion → LD A,H / LD IXH,A.
- **symbol+offset**: `LD DE, _data+20` works in MZA (expression evaluator handles +). Z80-VALIDATE warns but assembly succeeds.

### VIR Backend
- **OpAsmBlock**: CFG solver must copy AsmTemplate. Per-block solver had it, CFG didn't.
- **Grace INC dedup**: "dead before RET" optimization removes INC HL that computes return value. Fix: preserve INC/DEC on return-capable registers.
- **Tail call guard**: CALL+RET→JP restricted to local labels. External calls need proper parameter setup.
- **Post-emit validation**: Catches Z3 16-bit load clobber → PBQP fallback. Demoted to warning when pair aliasing covers it.
- **String pool ordering**: spliceVIRFallback: globals before strings. MZA trailing-zero trim clipped string data.
- **InlineTrivial drops labels**: Asm-body functions inlined → CALL target label removed → assembly error. Affects Pascal stdlib, ABAP seed SQL, mzv host overrides.

### ABAP Runtime
- **Target-aware lowering**: Set `hm.Target` BEFORE `l.lower()` via `LowerProgramWithTarget`. Guards needed for sel_register, sel_show, sel_get_int on ZX.
- **Seed SQL asm wrappers**: Each `*!sql` pragma gets its own asm function with hardcoded string address (mangled symbol: `@mir2.str.N` → `_mir2_str_N`).
- **Dynamic stmt handle**: `_itab_stmt_handle` global stores sqlite_query result. Hardcoded handle=1 broke when seed SQL consumed handle 1.
- **Column index in A not C**: PFCCO for `(u16, u8)` puts second param in A. Asm blocks must set A for column index, not C.
- **Inline I/O for column 1**: Second sqlite_column_text call needs inline OUT protocol to avoid A clobber from first call's print loop.

### ZX Spectrum
- mze spectrum target: no real ROM. Font must be embedded (768 bytes) or program uses ROM address $3D00 (MZX only).
- Console port $23: `IN A, ($23)` returns `0x80|byte` or `0x00`. Poll loop handles race with stdin goroutine.
- CLS: `LD (HL), 0x47 / LDIR` for all 768 attribute bytes. Sets white ink on black paper.
- Screen address interleaving: `H = $40 | (row & $18)`, `L = (row & 7) << 5 | col`.
- `_itab_print_*` needs `_zx_putchar` not BDOS on spectrum target.

### Cross-Session Coordination
- dedelulu session IDs change on reboot — always broadcast new ID.
- GPT-5.4 integration: `ddll ask gpt54 -s session @file.md "review this"` — persistent sessions with file injection.
- Five teams coordinating: minz, minz-vir, z80-optimizer, minz-abap, dedelulu.

---

## Research Programme

### Paper A: Register Allocation as a Solved Game
Data complete. Draft reviewed by GPT-5.4. 315 sigs, 97.8% convergence, phase transition, 88.2% transfer. Cross-compiler proof (Nanz=SDCC). Double phase transition (enumeration + feasibility).

### Paper B: Exact Inlining via Cost Oracle
Architecture designed (ADR-0040). DP on call graph. GPU table as exact cost oracle. 36% T-state savings. Island decomposition for >8v.

### Paper C: Compositional Register Allocation
Treewidth decomposition. 99.5% random / 46.3% dense corpus. Composition verified on 13.2M shapes (5.06T overhead). Self-hosting on Z80 (2.7K table, 40KB).

### ABI Paper: PFCCO vs Stack-Based
swap 20:0, minmax 63:11. Response to Philipp Krause (SDCC maintainer). Outparam detection promotes write-only pointers to tuple returns.
