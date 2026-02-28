# MinZ Iterator Implementation Status

*Updated: 2026-02-28*

## E2E Verified (MZX --console-io)

Full pipeline: MinZ source -> parser -> semantic -> Z80 codegen -> MZA -> MZX emulator.

| Test | Chain | Output | Hex |
|------|-------|--------|-----|
| `iter_foreach.minz` | `arr.forEach(console_log)` | `ABCDE` | `41424344450a` |
| `iter_take.minz` | `arr.iter().take(3).forEach(console_log)` | `ABC` | `4142430a` |

Run: `bash examples/e2e_iterators/run_e2e.sh`

## What Works

### Single-operation chains (E2E verified)

```minz
arr.forEach(console_log);            // DJNZ loop, direct CALL — verified
arr.iter().take(3).forEach(fn);      // counter = 3 instead of arr.length — verified
```

### Parser + semantic (63 unit tests pass, Z80 codegen WIP)

All 14 iterator operations parse correctly. `IteratorOp.Argument` separates
numeric args (take/skip) from function refs (map/filter/forEach):

| Operation | Parser | Semantic | Z80 E2E | Notes |
|-----------|--------|----------|---------|-------|
| `forEach(fn)` | pass | pass | **pass** | Single CALL in DJNZ loop |
| `take(n)` | pass | pass | **pass** | Limits DJNZ counter |
| `skip(n)` | pass | pass | **fail** | Counter correct, pointer offset dropped |
| `map(fn)` | pass | pass | **fail** | Return value doesn't flow to next op |
| `filter(fn)` | pass | pass | **fail** | Element clobbered by predicate result |
| `map(\|x\| expr)` | pass | pass | **fail** | Lambda param not loaded, body incomplete |
| `filter(\|x\| expr)` | pass | pass | **fail** | Same as filter(fn) + lambda issues |
| `peek(fn)` | pass | pass | untested | Same codegen path as forEach |
| `inspect(fn)` | pass | pass | untested | Same codegen path as forEach |
| `takeWhile(fn)` | pass | pass | untested | Generates early-exit jump |
| `enumerate()` | pass | pass | Z80 N/A | MIR OK, needs OpPush in Z80 backend |
| `reduce(fn)` | pass | pass | Z80 N/A | MIR OK, needs OpPush in Z80 backend |
| `reduce(init, fn)` | pass | pass | Z80 N/A | MIR OK, needs OpPush in Z80 backend |
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

**Fix:** Save/restore HL around every CALL inside the DJNZ loop body. Two options:

Option A — PUSH/POP HL (simple, +21 T-states per call):
```asm
    PUSH HL              ; Save array pointer
    CALL is_big
    POP HL               ; Restore array pointer
```

Option B — Use IX for the array pointer (frees HL for calls, +6 T-states per element):
```asm
    LD A, (IX+0)         ; Load element (19 T vs 7 T for LD A,(HL))
    INC IX               ; Advance pointer
```

Option A is simpler and doesn't change the overall loop structure. The element
register (C) also needs saving — either `PUSH BC` or reload from `(HL)` after
the call.

**Where:** `z80.go` — detect that we're inside a DJNZ iterator loop (the HL
hint is `RegHintHL`) and wrap CALL instructions with PUSH/POP HL. Also ensure
the element value flows correctly: after `CALL map_fn`, load the result
(from L or wherever) into A before calling the next function in the chain.

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

| Root Cause | Affects | Fix | Effort |
|---|---|---|---|
| Arithmetic op elision | skip offset, lambda bodies | Don't elide ADD/SUB in regalloc | Small |
| HL clobbered by CALL | filter, map, all chains | PUSH/POP HL around CALLs in DJNZ | Medium |
| Lambda param not stored | all lambda chains | Emit OpStore before inline body | Small |

Fixing Bug 2 (HL clobber) unblocks the most operations — filter, map,
and all multi-function chains would work. It's the highest-impact fix.

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
Assembly:
  LD HL, scores        ; array pointer
  LD B, 10             ; DJNZ counter
loop:
  PUSH HL              ; save pointer (Bug 2 fix)
  LD A, (HL)           ; load element
  CP 90                ; filter: x >= 90?
  JR C, skip
  ADD A, 5             ; map: x + 5
  CALL print_u8        ; forEach
skip:
  POP HL               ; restore pointer (Bug 2 fix)
  INC HL
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

### Test Suite (63 tests)

| Package | Tests | What |
|---------|-------|------|
| `pkg/parser/participle/` | 20 | Chain conversion, argument routing |
| `pkg/semantic/` | 17 | DJNZ eligibility, filter/map/lambda IR, take/skip/enumerate/reduce |
| `pkg/codegen/` | 5 | Z80 assembly patterns |
| `pkg/mirvm/` | 8 | DJNZ execution at MIR level |
| `tests/iterator_corpus/` | 18 | Full pipeline regression (.minz files) |
| `examples/e2e_iterators/` | 2 | MZX --console-io verified |

### Performance (when codegen bugs are fixed)

| Pattern | Traditional | Iterator | Speedup |
|---------|------------|----------|---------|
| Simple forEach | 45 T/elem | 18 T/elem | 2.5x |
| Map + Filter | 90 T/elem | 25 T/elem | 3.6x |
| Complex chains | 150+ T/elem | 30 T/elem | 5x+ |
