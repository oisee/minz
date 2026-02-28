# Flag-Based Boolean ABI for Iterator Predicates

*2026-02-28 — Report 013*

## Discovery

While analyzing the three Z80 codegen bugs blocking iterator chains (see
[Iterator Implementation Status](../docs/Iterator_Implementation_Status.md)),
we identified that Bug 2 (HL clobbered by CALL return) and the element-lost
problem share a single root cause: **boolean results don't need to be in HL**.

Iterator predicates (`filter`, `takeWhile`) return a boolean that is:
- Consumed exactly once (by `OpJumpIfNot`)
- Never stored to memory
- Never passed to another function

The Z80 `CP` instruction already sets flags that encode the comparison result.
Routing through `LD HL, 0/1` → `LD A, L` → `OR A` → `JP Z` is pure waste.

## The Optimization

### Before (current codegen — 6 instructions, HL destroyed)
```asm
    CALL is_big                ; HL = 1 or 0 (POINTER GONE)
    LD D, L                    ; Save boolean
    LD A, D                    ; Load to A
    OR A                       ; Test zero
    JP Z, filter_continue      ; Skip if false
    CALL console_log           ; A = boolean, not element!
```

### After — inlined predicate (2 instructions, HL untouched)
```asm
    LD A, (HL)                 ; Element in A
    CP 68                      ; x > 67? (CY=1 if A < 68)
    JR C, filter_continue      ; Skip if less than
    CALL console_log           ; A still has element!
```

### After — CALL predicate with flag return (4 instructions, HL preserved)
```asm
    PUSH HL                    ; Save array pointer
    CALL is_big                ; Sets CY flag, doesn't touch HL
    POP HL                     ; Restore pointer
    JR C, filter_continue      ; Direct flag test
```

## T-State Analysis

| Pattern | Current | Inlined | CALL+flag | Savings |
|---|---|---|---|---|
| Single filter | 45 T | 8 T | 28 T | 5.6x / 1.6x |
| filter.filter | 90 T | 16 T | 56 T | 5.6x / 1.6x |
| filter.map.forEach | 115 T | 38 T | 58 T | 3x / 2x |

The inlined path is the big win — for simple lambda predicates like `|x| x > 5`,
the comparison becomes a single `CP` + `JR` with zero register pressure.

## Impact on Iterator Chain Bugs

| Bug | Status with this fix |
|---|---|
| Bug 1: Arithmetic elision | Unaffected (separate fix needed) |
| Bug 2: HL clobbered by CALL | **SOLVED** for filter/takeWhile predicates |
| Bug 3: Lambda param not stored | **BYPASSED** for simple lambdas (inlined, no param needed) |

For simple filter lambdas (`|x| x > N`), this fix bypasses both Bug 2 and Bug 3
simultaneously — the comparison is inlined directly, no function call, no lambda
parameter storage, no HL clobber.

## Composability Example

```minz
scores.iter()
    .filter(|x| x >= 60)        // CP 60 / JR C, skip1
    .filter(|x| x < 100)        // CP 100 / JR NC, skip2
    .map(|x| x + 5)             // ADD A, 5
    .forEach(console_log)        // OUT ($23), A
```

Target assembly (entire chain, HL untouched throughout):
```asm
    LD HL, scores
    LD B, count
loop:
    LD A, (HL)            ; 7 T — load element
    CP 60                 ; 7 T — filter 1: x >= 60?
    JR C, skip            ; 7/12 T — skip if < 60
    CP 100                ; 7 T — filter 2: x < 100?
    JR NC, skip           ; 7/12 T — skip if >= 100
    ADD A, 5              ; 7 T — map: x + 5
    OUT ($23), A          ; 11 T — forEach: output
skip:
    INC HL                ; 6 T — next element
    DJNZ loop             ; 13/8 T — loop
```

**Total per element (both filters pass): 72 T-states.**
Current codegen (broken): would be 200+ T-states if it worked.

## Precedent

MinZ already uses flag-based returns for error handling:
- `@error` sets CY flag (`SCF`) — callers test with `JR C, error_handler`
- This is the same pattern, applied to boolean predicates

## Implementation Plan

See [ADR-0008](../docs/adr/0008-flag-based-boolean-abi-for-iterators.md) for
full architectural decision record.

### Phase 1: Inline simple lambda predicates (highest impact)
- Detect `filter(|x| x OP N)` pattern in semantic layer
- Emit `CP N` + conditional jump directly in DJNZ loop body
- No new IR opcodes needed — just pattern matching in codegen

### Phase 2: Flag-return convention for named predicates
- Add `OpCallPredicate` or annotation on `OpCall`
- Generate flag-return variant of predicate functions
- Wrap with PUSH/POP HL for pointer preservation

### Phase 3: Chain fusion
- Consecutive filters → consecutive CP + JR (no intermediate state)
- Filter + map → CP + JR + arithmetic (single pass)
- Wire into `pkg/optimizer/fusion.go`

## Decision

Accepted as the primary fix strategy for Bug 2 (HL clobber). Phase 1 (inline
lambda predicates) provides the highest impact with the smallest implementation
surface. ADR-0008 documents the full architectural decision.
