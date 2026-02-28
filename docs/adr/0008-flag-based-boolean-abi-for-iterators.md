# ADR-0008: Flag-Based Boolean ABI for Iterator Predicates

## Status
Proposed

## Context

The Z80 calling convention in MinZ returns all results in HL, including booleans
(`LD HL, 1` for true, `LD HL, 0` for false). This creates a critical problem
inside DJNZ iterator loops: HL is also the array pointer, so any `CALL` that
returns a boolean destroys the pointer.

Current `filter(is_big).forEach(console_log)` codegen:
```asm
    LD A, (HL)                 ; Load element from array
    LD C, A                    ; Element in C
    CALL is_big                ; Returns HL=1 (true) — POINTER GONE
    LD D, L                    ; D = 1 (filter result)
    LD A, D                    ; A = 1 (not the element!)
    OR A
    JP Z, filter_continue
    CALL console_log           ; Outputs 1, not the element
    INC HL                     ; Increments 1, not the pointer
```

Three problems: HL destroyed, element lost, 5 extra instructions for boolean routing.

Key insight: **iterator predicates never store their boolean result** — it's consumed
exactly once by `OpJumpIfNot` as a conditional jump. The HL round-trip is pure waste.

## Decision

Introduce a **flag-based boolean return convention** scoped to iterator predicates
(filter, takeWhile). Instead of returning `HL=0/1`, predicate functions return
their result via CPU flags (CY or Z), leaving HL untouched.

### Flag Convention

| Comparison | Z80 `CP` natural flag | Convention |
|---|---|---|
| `x > N` | CY=0 if A >= N+1 | `CP N+1` then `JR C, skip` |
| `x >= N` | CY=0 if A >= N | `CP N` then `JR C, skip` |
| `x < N` | CY=1 if A < N | `CP N` then `JR NC, skip` |
| `x <= N` | CY=1 if A <= N | `CP N+1` then `JR NC, skip` |
| `x == N` | Z=1 if A == N | `CP N` then `JR NZ, skip` |
| `x != N` | Z=0 if A != N | `CP N` then `JR Z, skip` |

### Target Assembly

**Simple predicates (inlined — no CALL at all):**
```asm
loop:
    LD A, (HL)             ; Load element
    CP 68                  ; filter: x > 67?
    JR C, skip             ; CY set = A < 68 = skip
    ADD A, 5               ; map: x + 5
    CALL print_u8          ; forEach
skip:
    INC HL                 ; HL never touched by filter!
    DJNZ loop
```

**Complex predicates (CALL with flag return):**
```asm
loop:
    LD A, (HL)             ; Load element
    LD C, A                ; Save element
    PUSH HL                ; Save pointer (just in case)
    CALL is_valid          ; Sets CY flag, does NOT touch HL
    POP HL                 ; Restore pointer
    JR C, skip             ; Direct flag test
    LD A, C                ; Restore element
    CALL process           ; Next chain op
skip:
    INC HL
    DJNZ loop
```

### Implementation Scope

This is **not a global ABI change**. It applies only when:
1. We're inside a DJNZ iterator loop body
2. The operation is `filter` or `takeWhile`
3. The predicate is either:
   - A lambda with a simple comparison body (`|x| x > 5`) — **inline** the comparison
   - A named function returning bool — generate a **flag-return variant**

### IR Changes

- New opcode `OpCallPredicate` (or annotation on `OpCall`) that signals "result is in flags"
- New opcode `OpJumpIfFlag` with `FlagSense` field (CY/NC/Z/NZ) replacing `OpJumpIfNot`
- Semantic layer marks filter/takeWhile predicates with `IsPredicateCall: true`

### Codegen Changes (z80.go)

For `OpCallPredicate`:
- Do NOT emit `LD HL, 0/1` in the called function — emit `RET` after `CP`
- Do NOT emit `storeFromHL(inst.Dest)` — result is in flags
- Emit `PUSH HL / CALL / POP HL` wrapper (pointer preservation)

For inlined lambda predicates:
- Detect `|x| x > N` pattern → emit `CP N+1` + `JR C, label` directly
- No function call, no register allocation, no HL clobber

## Consequences

### Positive
- **Solves Bug 2** (HL clobber) for filter/takeWhile — highest-impact codegen bug
- **Solves element-lost problem** — A/C still hold the element after the test
- **Massive T-state savings** for filter chains:
  - Current: ~45 T-states per filter (CALL + LD HL,0/1 + LD D,L + LD A,D + OR A + JP Z)
  - Inlined: ~8 T-states (CP + JR) — **5.6x speedup per filter**
  - With CALL: ~28 T-states (PUSH HL + CALL + CP + RET + POP HL + JR C)
- **Composable**: Multiple `.filter()` stages just chain CP + JR instructions
- **Natural Z80 idiom** — `CP` + conditional jump is how Z80 programmers write comparisons

### Negative
- Two boolean conventions (flag-based for iterators, value-based for everything else)
- Function reuse: a function used both as iterator predicate and normal boolean needs two variants (or a wrapper)
- More complex codegen path for iterator loop bodies

### Neutral
- Does not affect non-iterator code paths
- Error handling ABI (`@error` with CY flag) already establishes precedent for flag-based returns
- Lambda inlining is already partially implemented; this extends it with comparison awareness

## Alternatives Considered

### A: PUSH/POP HL around all CALLs in DJNZ
Simple but expensive (+21 T/call), doesn't solve element-lost problem, doesn't
reduce instruction count. Still needed as fallback for non-predicate calls (map, forEach).

### B: Use IX for array pointer
Frees HL for calls but `LD A, (IX+0)` is 19 T vs 7 T for `LD A, (HL)`.
+12 T per element access. Doesn't solve the boolean waste.

### C: Global flag-based boolean ABI
Too invasive — would break all non-iterator boolean usage. Booleans stored in
memory/variables need value representation. Iterator-scoped is the right granularity.

## References
- [Iterator Implementation Status](../Iterator_Implementation_Status.md) — Bug 2 analysis
- [ADR-0006](0006-ez80-adl-address-widening.md) — Related register pressure issue
- Z80 CPU manual: CP instruction flag behavior
- MinZ `@error` pattern — existing flag-based ABI precedent (CY flag)
