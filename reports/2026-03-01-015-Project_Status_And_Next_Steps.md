# MinZ Project Status & Next Steps

*2026-03-01 | MinZ v0.19.4 | Report #015*

---

## Current State (v0.19.4, March 2026)

### Toolchain — 9 Binaries, Pure Go, Zero Dependencies

| Tool | Status | Notes |
|------|--------|-------|
| MZC (compiler) | Production | ~90K LOC Go, 4 backends (Z80, 6502, C, Crystal) |
| MZA (assembler) | Production | Table-driven encoder, all Z80 ops including undocumented |
| MZE (emulator) | Production | 1335/1335 FUSE tests — gold-standard Z80 verification |
| MZX (ZX Spectrum) | Production | T-state accurate ULA, AY-3-8912 audio, profiler, .sna/.tap/.trd/.scl |
| MZD (disassembler) | Production | IDA-like analysis engine, xrefs, ROM tables, ABI propagation |
| MZRUN (remote) | Working | DZRP protocol, compiles + uploads to running emulator |
| MZR (REPL) | WIP | Interactive evaluation |
| MZV (VM) | WIP | Platform abstraction layer |
| MZTAP | Working | TAP file generation |

The entire toolchain builds with `make all` — no external assemblers, emulators, or third-party tools needed. One `make install-user` puts everything in `~/.local/bin/`.

### Language Features — Core Stable

**Working and tested:**
- Types: `u8`, `u16`, `i8`, `i16`, `bool`, `void`, pointers
- Functions: `fun`/`fn`, overloading, multiple returns, nested functions
- Control flow: `if`/`else`, `while`, `for i in 0..n`, `loop {}`
- Structs: declaration, field access, UFCS method syntax (`v.length()`)
- Enums: `enum State { IDLE, RUNNING }` with values, `State::IDLE` syntax
- Arrays: declaration, indexing
- Globals: `global counter: u8 = 0;`
- Lambdas: closure syntax, zero-cost transform to direct CALL
- String interpolation: `"Hello #{name}!"` (Ruby-style)
- Inline assembly: `asm { }` blocks, `asm fun` for pure-assembly functions
- CTIE: compile-time interface execution (trait monomorphization)
- True SMC: self-modifying code optimization (parameter patching into immediates)
- @extern FFI: `extern fun putchar(c: u8) at 0x10;` with RST optimization
- Operator overloading: `v1 + v2` via `impl` blocks
- Error propagation: `@error(code)` with CY flag ABI
- Metafunctions: @define, @print, @if/@elif/@else, @error, @lua
- Module system: `import stdlib.cpm.bdos;`
- 3 compilation targets: ZX Spectrum, CP/M, Agon Light 2

**In progress:**
- Iterator chains: 11 ops compile to Z80, but pointer-walk regression blocks E2E correctness
- Pattern matching: syntax parses, codegen partial
- MIR interpreter: arrays/structs working, not complete

### Test Suite — 19/19 Packages Pass

| Area | Tests | Status |
|------|------:|--------|
| Z80 emulator (FUSE) | 1335 | 1335 pass (100%) |
| z80testing E2E | 34 | 27 pass, 7 skip, 0 fail |
| Iterator unit tests | 53 | 53 pass (parser 18, semantic 20, codegen 7, MIR VM 8) |
| Iterator corpus | 18 | 18 compile to Z80 |
| Iterator E2E shell | 6 | 0 pass (pointer-walk regression) |
| Examples | ~272 | ~81% compile |
| All Go packages | 19 | 19 pass (codegen needs `-vet=off` — pre-existing) |

The test infrastructure is solid. Every compiler pipeline stage has dedicated tests. Known codegen bugs are documented with `t.Skipf` so they don't mask new regressions.

---

## What's Broken — Critical Codegen Bugs

These block real programs from producing correct output.

### 1. Iterator Pointer-Walk Regression

**Symptom:** `arr.forEach(console_log)` outputs `AAAAA` instead of `ABCDE` — every iteration reads the first array element.

**Root cause:** In `generateDJNZIteration()` (`pkg/semantic/iterator.go`), the `OpLoad` instruction that reads array elements uses `Src1=r8` (the base array pointer) instead of `Src1=r10` (the advancing pointer that gets `INC HL` each iteration).

**Evidence:** Generated Z80 assembly shows:
```asm
; BUG: line 34 always reads from $F010 (base pointer)
LD HL, ($F010)    ; Virtual register 8 — ALWAYS the start of array
LD A, (HL)        ; Always reads first element

; ... call, PUSH/POP HL ...

; Line 48-50: register 10 correctly incremented
LD HL, ($F014)    ; Virtual register 10 from memory
INC HL            ; Advances to next element
LD ($F014), HL    ; Saved back — but never read by OpLoad above
```

**Fix:** Change one register reference in the IR emit — `OpLoad.Src1` should be the pointer copy register, not the original array register.

**Impact:** Blocks all 6 iterator E2E tests. All unit tests pass because they validate IR structure (correct), not execution output.

**Difficulty:** Small — likely a 1-line fix in `iterator.go`.

### 2. Register Allocator in While/For Loops

**Symptom:** Functions with while-loops and 2+ live variables produce wrong results. `max(10, 5)` returns 0 instead of 10. `strlen("Hello")` returns 0 instead of 5.

**Root cause:** The register allocator assigns the same physical register to two virtual registers that are both live across a loop back-edge. When one is written, the other is silently clobbered.

**Evidence:** Multiple E2E tests fail with result=0 for functions containing `while` loops. Simple non-loop functions work correctly.

**Fix:** Requires auditing liveness intervals at loop back-edges. The allocator needs to extend live ranges for variables that are read after a loop iteration writes to a different variable in the same physical register.

**Impact:** Blocks most non-trivial programs — any function with a while/for loop and multiple variables hits this.

**Difficulty:** Large — this is the single biggest quality issue in the compiler. ADR-0006 and ADR-0007 document aspects of it. Partially mitigated by constant-map invalidation at labels, but the core issue remains.

### 3. Inline Filter Constant Not Tracked

**Symptom:** `filter(|x| x > 3)` passes all elements through instead of only those > 3. The generated Z80 code compares against 0 instead of 3.

**Root cause:** The `constantValues` map in `z80.go` loses the entry for the filter threshold between the `OpLoadConst` that sets it and the `OpJumpIfFlag` that consumes it. By the time codegen reaches `OpJumpIfFlag`, it can't find the constant and falls back to `OR A` (compare against 0).

**Evidence:** Generated assembly shows `OR A` (test zero) where `CP 4` (compare 4, since > 3 means >= 4) is expected.

**Fix:** Either embed the constant directly in `OpJumpIfFlag.Imm2` at IR-emit time in the semantic analyzer, or fix the `constantValues` map to not lose entries between these two adjacent instructions.

**Impact:** All inline lambda filters pass everything through — they still iterate correctly, just don't filter.

**Difficulty:** Medium.

---

## What's Broken — Medium Codegen Bugs

These produce correct results but waste cycles or use wrong registers.

### 4. OpPush Routes Through HL

**Symptom:** `PUSH` always loads the source to HL first, then does `PUSH HL`. For enumerate/reduce with values in BC or DE, both pushes become `PUSH HL`, pushing the wrong values.

**Fix:** Check if source is already in BC/DE/HL and emit `PUSH BC`/`PUSH DE` directly.

### 5. Counter Waste (~12 T-states/iteration)

**Symptom:** Every DJNZ loop iteration has redundant `LD A, B` / `LD B, A` register shuffling because the register allocator maps B through A as an intermediary.

**Fix:** Peephole pattern to collapse the shuffle, or teach the allocator to use B directly.

### 6. Unused Result Saves

**Symptom:** After every `CALL` to a void function, codegen saves the return value (`LD D, L`) even though nobody reads it. Wastes 4 T-states per call.

**Fix:** Peephole pattern to remove dead stores after void calls, or codegen check to skip the store when `Dest=0` or unused.

---

## What's Missing

| Feature | Status | Phase |
|---------|--------|-------|
| Pattern matching codegen | Syntax parses, codegen partial | Phase 3 |
| Generator syntax (gen/yield) | Not started — design doc exists | Phase 3 |
| Fusion optimizer | Skeleton in `pkg/optimizer/fusion.go`, not wired into pipeline | Phase 2 |
| Superoptimizer peephole | 602K proven rules (ADR-0009), not integrated | Phase 2 |
| LSP server | Not started | Phase 5 |
| DAP debugger | Not started | Phase 5 |
| WASM playground | Not started | Phase 5 |

---

## Recommended Next Steps

### Immediate — High Impact, Small Effort

#### Step 1: Fix pointer-walk regression
- **File:** `pkg/semantic/iterator.go`, `generateDJNZIteration()`
- **Change:** `OpLoad.Src1` from base array register to advancing pointer register
- **Unblocks:** All 6 iterator E2E tests
- **Effort:** ~1 line change + rebuild + verify

#### Step 2: Fix inline filter constant
- **File:** `pkg/codegen/z80.go` or `pkg/semantic/iterator.go`
- **Change:** Embed constant in `OpJumpIfFlag.Imm2` at IR emit, or fix `constantValues` tracking
- **Unblocks:** `filter(|x| x > N)` producing correct filtered output
- **Effort:** Small-medium

#### Step 3: Wire remaining E2E tests
- **File:** `examples/e2e_iterators/run_e2e.sh`
- **Change:** Add iter_skip, iter_map_foreach, iter_filter_foreach, iter_lambda_map to the runner
- **Unblocks:** Full 6/6 E2E validation (after Step 1)
- **Effort:** Small

### Near-Term — Phase 1 Completion

#### Step 4: Register allocator verification
- The single biggest quality issue in the compiler
- While-loop programs (`max`, `strlen`, `count_to`, `bubble_sort`) all produce wrong results
- Need to audit liveness intervals at loop back-edges
- ADR-0006 (address widening) and ADR-0007 (newline handling) document known aspects
- **Effort:** Large — this is the most impactful and most difficult remaining work

#### Step 5: OpPush register routing
- **File:** `pkg/codegen/z80.go`
- **Change:** Direct `PUSH BC`/`PUSH DE` instead of always loading to HL first
- **Unblocks:** enumerate/reduce correctness
- **Effort:** Small

### Medium-Term — Phase 2-3

#### Step 6: Fusion optimizer
Wire `pkg/optimizer/fusion.go` into the MIR pipeline. The skeleton exists with detection logic scaffolding — needs actual fusion implementation to merge multi-stage iterator chains into single loops.

#### Step 7: Superoptimizer peephole rules
602K proven Z80 optimizations from ADR-0009. Need to select the most impactful rules, convert to peephole patterns, and integrate with the existing 35+ patterns.

#### Step 8: Pattern matching codegen
Complete match/case statement code generation. Syntax already parses. Need exhaustiveness checking and jump-table optimization for enum matching.

#### Step 9: Generator syntax
Implement `gen`/`yield` for lazy iteration. Design document exists. Requires stack frame management for suspended generators and integration with the iterator chain pipeline.

---

## Priority Summary

```
NOW:    [1] pointer-walk fix  →  [2] filter constant  →  [3] wire E2E tests
NEXT:   [4] register allocator (the big one)  →  [5] OpPush routing
LATER:  [6] fusion  →  [7] superoptimizer  →  [8] pattern matching  →  [9] generators
FUTURE: LSP  →  DAP  →  WASM playground
```

Steps 1-3 are quick wins that prove the iterator pipeline works end-to-end. Step 4 is the long pole — fixing it unlocks the majority of currently-broken programs. Steps 6-9 are feature work that builds on a stable foundation.

---

## References

- [GenPlan](../docs/GenPlan.md) — canonical development roadmap
- [Iterator E2E Testing Report (#014)](2026-03-01-014-Iterator_E2E_Testing_Report.md) — detailed test results
- [Iterator Implementation Status](../docs/Iterator_Implementation_Status.md) — pipeline details
- [ADR-0006](../docs/adr/ADR-0006-address-widening.md) — address widening
- [ADR-0007](../docs/adr/ADR-0007-newline-handling.md) — newline handling
- [ADR-0008](../docs/adr/0008-flag-based-boolean-abi-for-iterators.md) — flag-based boolean ABI
- [ADR-0009](../docs/adr/ADR-0009-superoptimizer-peephole-rules.md) — superoptimizer rules
