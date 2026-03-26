# Session Wisdom: Birthday Marathon (Sessions 3-9)

**Dates:** 2026-03-24 to 2026-03-26
**Commits:** ~80 across main repo + VIR + z80-optimizer
**Release:** v0.23.0 Birthday Marathon Release

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

### 9. @error — CY Flag Error Propagation (Session 8)
`@error(N)` → `SCF / LD A, N / RET` (2 bytes). `@propagate` → `RET C` (1 byte!).
Implemented as built-in metafunction — zero parser/AST/semantic changes. 55 LOC.
Design insight: `?` in function name = fallible (parser enforcement, not solver).
CY liveness = just "require @check immediately after fallible call" — no Z3 needed.

### 10. Frontend Sprint (Session 8)
- PL/M: 5 examples created, all compile. MOD operator missing in HIR (filed).
- Lizp: all 8 examples pass (MZA fixes from session 4 resolved gaps).
- Pascal: SwitchStmt traversal fixed. Stdlib blocked on InlineTrivial dropping labels (VIR P6).
- InlineTrivial asm-body label drop: affects Pascal stdlib, ABAP seed SQL, mzv hosts. ONE backend fix covers all.

### 11. Double Phase Transition (Session 7-8)
- Enumeration cliff at ~16 register locations (Paper A original)
- Feasibility cliff at 6v: only 0.9% of shapes feasible!
- 99.1% of 6v shapes are PROVABLY IMPOSSIBLE on Z80
- Treewidth analysis: 99.5% random graphs tractable, 46.3% for dense corpus
- Composition verified on 13.2M shapes (5.06T avg overhead)

### 12. C17 Conformance + C23 Extensions (Session 9)
First Z80 compiler with C11/C17 conformance. SDCC doesn't even have C11.
- **5 libc headers:** stdbool.h, assert.h, ctype.h (17 inline funcs, zero LUT), stdalign.h, stdnoreturn.h
- **C11:** anonymous structs/unions (field promotion via FieldOffset()), `_Alignof` → 1, `typeof`
- **C23:** `bool`/`true`/`false` as keywords, `nullptr`, `#embed "file" limit(N) offset(N)`
- **`#embed`** = the Z80 killer: binary include for sprites, fonts, GPU lookup tables
- **Array designated init:** `uint8_t arr[5] = {[2] = 42, [4] = 99}`
- **Global array init fix:** const LUTs from brace initializers now populate correctly (was all-zeros)
- **524/524 corpus asserts** (350 c89 + 174 c99+), 13 test files
- `docs/C_Standards_Roadmap.md` — full C99→C23 feature matrix

### 13. @error Layer 2 — `?` Parser Enforcement (Session 9)
`fun safe_div?(a: u8, b: u8)` — `?` in function name marks fallible.
Parser enforces: `@check`/`@propagate` MUST follow every `?`-call. Missing → compile error.
Lexer change: trailing `?` allowed in identifiers. `sanitizeIdent()` maps `?` → `_` for asm labels.
35/35 Nanz examples still compile. VIR 5/5 Z3.

### 14. RLCA Sled — Multi-Entry Barrel Shifter (Session 9)
9 bytes, 8 entry points. `CALL __rotate_4` = nibble swap. No loop, no counter.
```z80
__rotate_7:  RLCA    ; fall through ↓
...
__rotate_4:  RLCA    ; nibble swap entry
...
__rotate_0:  RET     ; 9 bytes total
```
Assembly peephole: 3+ consecutive RLCAs → `CALL __rotate_N`. Sled auto-emitted.
TSMC variant: first call patches CALL target → zero-overhead dispatch forever.

### 15. MZA INCBIN — Binary Data Embedding (Session 9)
`INCBIN "file.bin" [, offset [, length]]` — include binary verbatim.
Parser + directive handler. Resolves paths relative to source. Bounds checking.
Unlocks GPU lookup tables: mulopt8.bin, divopt8.bin, regalloc entries.

### 16. antique-toy Book Collaboration (Session 9)
New session (eo29c66e) joined for Z80 demoscene book.
Sent `_in/minz_v023_highlights.md` with: INCBIN, RLCA sled, DD prefix gotcha, GPU arithmetic, C23, @error.

### 17. GPT-5.4 + Gemini Integration via dedelulu
`ddll ask gpt54 -s session @file.md "review"` — persistent LLM sessions with file injection.
Paper A reviewed by GPT-5.4. @error design reviewed. Cross-LLM consensus on publishability.

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

### @error as Metafunction (not language feature)
- `@error(N)` = pure asm expansion: `SCF / LD A, N / RET`. Zero compiler changes.
- CY liveness doesn't need Z3 modeling — just parser enforcement: `@check` must follow fallible call.
- `?` in function name is convention, not syntax. Parser checks it, solver doesn't see it.
- `RET C` for propagation = Z80's NATIVE error mechanism. 1 byte, 5T. No other arch has this.
- Dead code after `@error(N)` is harmless — Z80 never reaches it, peephole can strip later.
- Design reviewed by GPT-5.4 + VIR. Both approve metafunction approach.

### InlineTrivial Label Drop (the recurring bug)
Affects: Pascal stdlib (WriteCrLf, WriteStr), ABAP seed SQL, mzv host overrides.
Root cause: InlineTrivial inlines small asm-body functions → CALL target label removed.
The function code is inlined but the LABEL disappears from the assembly output.
Other code that CALLs by label fails at assembly time.
Fix (VIR P6): ClobberAll=true guard prevents inlining of asm-body functions.
This ONE fix resolves issues in 3 frontends.

### Pascal moduleReferences Bug
stmtReferences didn't traverse SwitchStmt (CASE branches) or VarDeclStmt.
WriteLn calls inside CASE blocks weren't detected → runtime functions not emitted.
Fixed by adding cases to stmtReferences. But the emit still fails because
InlineTrivial drops the emitted function labels (see above).

### C Frontend (Session 9)
- **modernc.org/cc already handles most C99/C11** — for-init-decl, _Bool, _Generic, _Static_assert, typeof, _Alignof all parsed. Gap was just missing libc headers + predefined macros.
- **`FieldOffset()` from cc parser** is authoritative for struct field offsets. Our manual `resolveFieldOffset()` via MIR2 struct was wrong for anonymous members. Always prefer cc parser's offset.
- **Global array init was broken** — `evalConstInit()` only handled single values. Brace initializer lists for arrays needed manual byte population in the global lowerer. Cost: hours of "why are my LUTs all zeros?"
- **`#embed` as preprocessor text replacement** — simplest approach. Replace `#embed "file"` with comma-separated hex bytes BEFORE cc.Translate() sees the source. No parser changes needed.
- **ctype.h inline functions** — 17 functions, zero lookup table. Each is a pure leaf → VIR Z3 optimizes well. No ROM dependency.
- **MIR2 VM uses u16 arithmetic** — u8 overflow tests (swap_nibbles, mul8 wrapping) fail on mir2, work on z80. Use `via z80` for overflow-dependent asserts.

### RLCA Sled Design (Session 9)
- **Multi-entry fall-through** is the key insight. One function, N entry points. 9 bytes serve all 8 rotations.
- **Peephole folding threshold: 3+ RLCAs** → CALL __rotate_N. Below 3, inline is smaller (no CALL overhead).
- **SLA ≠ RLCA** — don't confuse shifts with rotations. SLA shifts 0 into bit 0, RLCA rotates bit 7 to bit 0. Sled is for rotation only.
- **RLD/RRD are slower** than 4×RLCA for nibble swap (18T each + memory setup vs 16T total).
- **Sled auto-emission** — detect `CALL __rotate_` in output string after peephole pass, emit sled if found. No manual flag needed.

### Cross-Session Coordination
- dedelulu session IDs change on reboot — always broadcast new ID.
- GPT-5.4 integration: `ddll ask gpt54 -s session @file.md "review this"` — persistent sessions with file injection.
- Five teams coordinating: minz, minz-vir, z80-optimizer, minz-abap, dedelulu.
- Gemini also available via `ddll ask gemini`.

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
