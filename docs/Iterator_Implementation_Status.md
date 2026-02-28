# MinZ Iterator Implementation Status

*Updated: 2026-02-28*

## E2E Verified (MZX --console-io)

Full pipeline: MinZ source -> parser -> semantic -> Z80 codegen -> MZA -> MZX emulator.

| Test | Chain | Output | Hex |
|------|-------|--------|-----|
| `iter_foreach.minz` | `arr.forEach(console_log)` | `ABCDE` | `41424344450a` |
| `iter_take.minz` | `arr.iter().take(3).forEach(console_log)` | `ABC` | `4142430a` |
| `iter_skip.minz` | `arr.iter().skip(2).forEach(console_log)` | `CDE` | `4344450a` |
| `iter_map_foreach.minz` | `arr.iter().map(add_one).forEach(console_log)` | `BCDEF` | `4243444546` |
| `iter_filter_foreach.minz` | `arr.iter().filter(\|x\| x > 67).forEach(console_log)` | `DE` | `4445` |
| `iter_lambda_map.minz` | `arr.iter().map(\|x\| x + 1).forEach(console_log)` | `BCDEF` | `4243444546` |

Run: `bash examples/e2e_iterators/run_e2e.sh`

### Compile-Time Diagnostics (v0.19.2)

Use `--compile-trace` to see every optimization decision:
```bash
mz program.minz --compile-trace 2>&1 | grep '\[semantic\]'
```
Output:
```
[semantic ] DJNZ loop: array[5] (≤255 threshold)
[semantic ] Inline filter: CP 68 + JR C (saved ~27 T-states/iter)
[semantic ] Lambda extracted: iter_lambda_main_0
```

## What Works

### Single-operation chains (E2E verified)

```minz
arr.forEach(console_log);            // DJNZ loop, direct CALL — verified
arr.iter().take(3).forEach(fn);      // counter = 3 instead of arr.length — verified
```

### Parser + semantic (63 unit tests pass)

All 14 iterator operations parse correctly. `IteratorOp.Argument` separates
numeric args (take/skip) from function refs (map/filter/forEach):

| Operation | Parser | Semantic | Z80 E2E | Notes |
|-----------|--------|----------|---------|-------|
| `forEach(fn)` | pass | pass | **pass** | Single CALL in DJNZ loop |
| `take(n)` | pass | pass | **pass** | Limits DJNZ counter |
| `skip(n)` | pass | pass | **pass** | Pointer offset via ADD HL |
| `map(fn)` | pass | pass | **pass** | PUSH/POP HL preserves pointer across CALL |
| `filter(\|x\| x > N)` | pass | pass | **pass** | Inline `CP N+1 + JR C` — no CALL, HL safe |
| `map(\|x\| x + N)` | pass | pass | **pass** | Inline lambda → `ADD A, N` |
| `filter(fn)` | pass | pass | untested | Named predicate (needs flag-return ABI) |
| `peek(fn)` | pass | pass | untested | Same codegen path as forEach |
| `inspect(fn)` | pass | pass | untested | Same codegen path as forEach |
| `takeWhile(fn)` | pass | pass | untested | Generates early-exit jump |
| `enumerate()` | pass | pass | **compiles** | OpPush/OpPop implemented (v0.19.2), needs E2E verify |
| `reduce(fn)` | pass | pass | **compiles** | OpPush/OpPop implemented (v0.19.2), needs E2E verify |
| `reduce(init, fn)` | pass | pass | **compiles** | OpPush/OpPop implemented (v0.19.2), needs E2E verify |
| `skipWhile(fn)` | pass | pass | N/A | Not implemented in DJNZ mode |
| `zip(other)` | pass | — | — | Parser only |
| `flatMap(fn)` | pass | — | — | Parser only |
| `collect()` | pass | — | — | Parser only |

## What Doesn't Work and Why

Three root causes block multi-operation chains. All are in the Z80 codegen
register allocator (`pkg/codegen/z80.go`), not in the parser or semantic layers.

### Bug 1: "Already in register" elision on arithmetic ops

**Affects:** `skip(n)` pointer offset, lambda `|x| x + 1` body

The register allocator tracks which virtual register is in which physical
register. When it sees `OpAdd(dest=r10, src1=r10, src2=r11)` and r10 is
already in HL, it emits `; Register 10 already in HL` and **skips the ADD
instruction entirely**. This optimization is correct for `OpMove` but wrong
for arithmetic — the value needs to be *modified*, not just *loaded*.

**Observed in `skip(2)` assembly:**
```asm
    ; Skip offset = 2 elements      ← IR emitted OpLoadConst + OpAdd
    ; Pointer to first element after skip
    ; Register 10 already in HL     ← codegen elides the ADD HL, DE
```

Counter is correct (3 iterations) but HL still points at element 0.

**Fix:** In the `OpAdd`/`OpSub` handler in `z80.go`, skip the "already in
register" elision. Only elide for `OpMove`/`OpLoad`. The check is roughly:

```go
// Before emitting: if dest already holds src1, skip load
// BUT: for ADD/SUB, we still need the arithmetic operation!
if inst.Op == ir.OpAdd || inst.Op == ir.OpSub {
    // Always emit the operation, even if dest == src1 register
}
```

### Bug 2: HL clobbered by CALL return value inside DJNZ loop

**Affects:** `filter(fn).forEach()`, `map(fn).forEach()`, all multi-function chains

The DJNZ loop uses HL as the array pointer (`LD A, (HL)` / `INC HL`). But
the Z80 calling convention returns results in HL (`LD HL, 1` for true,
`LD HL, result` for u8). Any `CALL` inside the loop body **destroys the
array pointer**.

**Observed in `filter(is_big).forEach(console_log)`:**
```asm
    LD A, (HL)                 ; Load element from array
    LD C, A                    ; Element in C
    CALL is_big                ; Returns HL=1 (true) — POINTER GONE
    LD D, L                    ; D = 1 (filter result)
    LD A, D                    ; A = 1 (filter result, not element!)
    OR A
    JP Z, filter_continue
    CALL console_log           ; Outputs A=1, not the element
    ...
    INC HL                     ; Increments 1, not the array pointer
```

Two sub-problems:
- **HL destroyed**: `INC HL` after the call increments the return value, not the pointer
- **Element lost**: A holds the filter boolean, not the original element, when `console_log` is called

**Fix: Flag-Based Boolean ABI** (see [ADR-0008](adr/0008-flag-based-boolean-abi-for-iterators.md))

The key insight: **iterator predicates never store their boolean result** — it's
consumed exactly once by `OpJumpIfNot`. The `LD HL, 0/1` round-trip is pure waste.
The Z80 `CP` instruction already sets flags that encode the result. Use them directly.

**Option C — Inline lambda predicates (best, no CALL at all):**
```asm
    LD A, (HL)           ; Load element
    CP 68                ; filter: x > 67? (CY=1 if A < 68)
    JR C, skip           ; Direct flag test — HL untouched!
    CALL console_log     ; A still holds the element
skip:
    INC HL               ; Pointer intact
    DJNZ loop
```

For `filter(|x| x > N)`, the lambda body is a single `CP` + conditional jump.
No function call, no HL clobber, no element lost. **8 T-states vs 45 T-states.**

**Option B — Flag-return convention for named predicates:**
```asm
    PUSH HL              ; Save array pointer
    CALL is_big          ; Sets CY flag, does NOT touch HL
    POP HL               ; Restore pointer
    JR C, skip           ; Direct flag test
```

The predicate function returns via flags instead of `LD HL, 0/1`.

**Option A — PUSH/POP HL (fallback for non-predicate CALLs like map/forEach):**
```asm
    PUSH HL              ; Save array pointer
    CALL double_u8       ; Returns result in HL (non-boolean)
    POP HL               ; Restore array pointer
```

Still needed for `map(fn)` and `forEach(fn)` which return actual values in HL.

**Precedent:** MinZ `@error` already uses flag-based returns (CY flag = error).
This extends the same pattern to boolean predicates.

**Where:** Semantic layer detects `filter(|x| x OP N)` pattern → inlines `CP`
directly. For named predicates, `z80.go` generates flag-return function variant.
See [Report 013](../reports/2026-02-28-013-Flag_Based_Boolean_ABI_Iterator_Predicates.md).

### Bug 3: Inlined lambda parameter not loaded

**Affects:** `map(|x| x + 1)`, `filter(|x| x > 5)`, all lambda chains

When a lambda is inlined into the DJNZ loop, the lambda body reads its
parameter from SMC address `$F000`. But the loop body never stores the
current element there.

**Observed in `map(|x| x + 1)` assembly:**
```asm
    LD A, (HL)             ; Load element into A
    LD C, A                ; Element in C
    ; Inlined lambda body:
    LD HL, ($F000)         ; Load "x" from $F000... but nobody stored it!
    LD ($F000), HL         ; No-op store
    LD HL, ($F000)         ; Load again (still garbage)
    LD C, L                ; C = garbage
    CALL console_log       ; Output garbage
```

Also, the `+ 1` expression is missing from the lambda body — the `OpAdd`
with `Imm: 1` is dropped (same Bug 1 elision).

**Fix:** In `generateIteratorLambda()` (`pkg/semantic/iterator.go`), emit
a store instruction before inlining the lambda body:

```go
// Store current element to lambda parameter's SMC address
irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
    Op:      ir.OpStore,
    Dest:    lambdaParamReg,  // SMC address for the parameter
    Src1:    elementReg,       // Current element from the loop
    Comment: "Store element to lambda parameter",
})
```

This ensures `$F000` contains the current element when the lambda body
reads it. Combined with the Bug 1 fix, the `+ 1` would also be emitted.

## Summary

| Root Cause | Affects | Status | Fix |
|---|---|---|---|
| Arithmetic op elision | skip offset, lambda bodies | **Mitigated** | Inline lambdas bypass the issue; named fn path still affected |
| HL clobbered by CALL | filter(fn), map(fn), chains with CALL | **Partially fixed** | Inline filter uses `CP + JR` (no CALL); named fn needs PUSH/POP HL |
| Lambda param not stored | complex lambda chains | **Bypassed** | Simple lambdas inline directly; complex ones still affected |

**What's working now (v0.19.2):** Inline lambda predicates (`|x| x > N`) compile
to `CP N+1 + JR C` — no function call, no HL clobber, no lambda param storage.
This is the flag-based boolean ABI from [ADR-0008](adr/0008-flag-based-boolean-abi-for-iterators.md).

**Remaining:** Named predicates (`filter(is_big)`) still need PUSH/POP HL and
flag-return convention. OpPush/OpPop handlers needed in Z80 backend for
enumerate and reduce.

## Architecture

### Compilation Pipeline
```
Source Code:
  scores.iter()
    .filter(|x| x >= 90)
    .map(|x| x + 5)
    .forEach(print_u8)
    |
AST: IteratorChainExpr
    |
Semantic: Type checking + operation dispatch
    |
IR: Virtual register instructions (OpLoad, OpCall, OpDJNZ, ...)
    |
Z80 Codegen: Physical register allocation + assembly emission
    |
Assembly (with flag-based boolean ABI — ADR-0008):
  LD HL, scores        ; array pointer
  LD B, 10             ; DJNZ counter
loop:
  LD A, (HL)           ; load element
  CP 90                ; filter: x >= 90? (flag-based, no CALL!)
  JR C, skip           ; CY set = A < 90 = skip
  ADD A, 5             ; map: x + 5
  CALL print_u8        ; forEach (PUSH/POP HL for non-bool CALLs)
skip:
  INC HL               ; HL untouched by filter!
  DJNZ loop
```

### Key Files

| File | Role |
|------|------|
| `pkg/ast/iterator.go` | IteratorChainExpr, IteratorOp (Function + Argument fields) |
| `pkg/parser/participle/convert.go` | `tryConvertIteratorChain()` — CallExpr -> IteratorChainExpr |
| `pkg/semantic/iterator.go` | Type checking, DJNZ/indexed dispatch, lambda extraction |
| `pkg/semantic/iterator_enhanced.go` | Enhanced DJNZ with take/skip/enumerate/reduce |
| `pkg/codegen/z80.go` | Z80 register allocation + assembly emission (bugs live here) |
| `pkg/optimizer/fusion.go` | Fusion framework (skeleton, not wired in) |

### Test Suite (63 unit + 18 corpus + 6 E2E)

| Package | Tests | What |
|---------|-------|------|
| `pkg/parser/participle/` | 20 | Chain conversion, argument routing |
| `pkg/semantic/` | 17 | DJNZ eligibility, filter/map/lambda IR, take/skip/enumerate/reduce |
| `pkg/codegen/` | 5 | Z80 assembly patterns (use `-vet=off`) |
| `pkg/mirvm/` | 8 | DJNZ execution at MIR level |
| `tests/iterator_corpus/` | 18 | Full pipeline regression (.minz files) |
| `examples/e2e_iterators/` | 6 | MZX --console-io verified (forEach, take, skip, map, filter, lambda map) |

### Performance (when codegen bugs are fixed)

| Pattern | Traditional | Iterator | Speedup |
|---------|------------|----------|---------|
| Simple forEach | 45 T/elem | 18 T/elem | 2.5x |
| Map + Filter | 90 T/elem | 25 T/elem | 3.6x |
| Complex chains | 150+ T/elem | 30 T/elem | 5x+ |

---

## GenPlan: Next Steps

### Phase 1: OpPush/OpPop in Z80 Backend -- DONE (v0.19.2)
- Added `case ir.OpPush:` / `case ir.OpPop:` to `generateInstruction()` in `z80.go`
- OpPush: load src to HL, then `PUSH HL`
- OpPop: `POP HL`, store to dest
- **Result:** 18/18 corpus tests now compile to Z80 (was 16/18)

### Phase 2: PUSH/POP HL around CALLs in DJNZ loops
- For `map(fn)` and `forEach(fn)` inside iterator loops, wrap CALL with `PUSH HL` / `POP HL`
- Semantic layer marks iterator CALL instructions with a hint (e.g., `inst.Meta["iter_call"] = true`)
- Z80 codegen checks hint and emits PUSH/POP around the CALL
- **Effort:** Medium (~50 lines semantic + codegen)

### Phase 3: Flag-return ABI for Named Predicates
- For `filter(is_big)` with named function, generate flag-return variant
- Predicate returns via CY flag instead of LD HL, 0/1
- Codegen: `PUSH HL` / `CALL is_big` / `POP HL` / `JR C, skip`
- **Effort:** Medium

### Phase 4: Fusion Optimizer Wiring
- Wire `pkg/optimizer/fusion.go` into the optimization pipeline
- Detect adjacent iterator ops that can be merged into single loop body
- Prerequisite: Phases 1-3 complete so fused code can actually generate correct Z80
- **Effort:** Large

### Phase 5: collect() + Generator Syntax
- `collect()` — allocate result array, store filtered/mapped elements
- `gen`/`yield` — lazy generator functions
- **Effort:** Large, future
