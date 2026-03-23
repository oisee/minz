# Report 108: Frontend Convergence — Stream, S-expr Features, MinZ Corpus Unification

**Date:** 2026-03-23 (evening session)
**Scope:** stdlib, Lanz, Lizp, Nanz parser, MinZ corpus cleanup, impl blocks

---

## Summary

This session focused on frontend convergence: making all MinZ frontends (Nanz, Lanz, Lizp, Frill) more capable, and bringing the legacy MinZ corpus under the modern HIR->MIR2->PBQP pipeline. 12 commits pushed.

## What Was Built

### 1. Stream Abstraction (`stdlib/core/stream.nanz`)

Unified write interface for Z80 targets:

- **BufStream** — write to in-memory buffer with capacity check
- **NullStream** — discard output, count bytes written
- Helpers: `bufstream_write_dec_u8`, `bufstream_write_str`
- 4 E2E asserts via MIR2 VM

Design: no function pointers, no vtables. Each stream type has its own concrete functions. Runtime dispatch via SMC patching planned as future `@smc` annotation.

### 2. Lanz (S-expr HIR): Lambda, Let-in, Match

Three new expression forms in the S-expression compiler:

| Form | Syntax | Desugars to |
|------|--------|-------------|
| **lambda** | `(lambda ((x u8)) u8 (return (+ x x)))` | Named `lambda_N` function + `AddrOfExpr` |
| **let-in** | `(let-in name ty init body)` | `hir.LetInExpr` (scoped binding) |
| **match** | `(match expr (0 body0) (1 body1) (_ default))` | Nested `CondExpr` chain |

5 E2E tests, all verified via MIR2 VM.

### 3. Lizp (Scheme/Lisp): fn, let*, case

Scheme-style sugar over the new Lanz forms:

| Lizp | Lanz equivalent |
|------|-----------------|
| `(fn params retty body)` | `(lambda ...)` |
| `(let* ((name ty val) ...) body)` | Nested `(let-in ...)` chain |
| `(case expr (val body) ...)` | `(match ...)` |

3 E2E tests + `functional.lizp` showcase (11 asserts: lambda + let* + case combined). 102 bytes Z80 binary.

### 4. impl Blocks in Nanz

```nanz
impl Drawable for Circle {
    fun draw(self) -> u8 {
        return self.radius * 6
    }
}
// Desugars to: fun Circle_draw(self: ^Circle) -> u8 { ... }
// Registered in methodTable for UFCS: circle.draw()
```

- `self` auto-typed as `^TypeName` with full field access
- Multiple methods per block
- Extra params after self supported
- 3 E2E tests (trait-for-type, multi-method, with-params)

### 5. MinZ Corpus Unification

Systematic cleanup to make legacy MinZ examples parseable through the Nanz pipeline:

| Change | Files | Occurrences |
|--------|-------|-------------|
| `let mut` -> `var` | 24 | 102 |
| Trailing `;` removed | 106 | ~2900 |
| `; }` and `; //` cleaned | 62 | 730 |
| `*u8` -> `^u8` pointer syntax | 23 | 60 |

**Result: 58/119 MinZ files (49%) now parse through Nanz** (was 37/119 = 31% at session start).

### 6. Wider Integer Types

`u24`, `i24`, `u32`, `i32` added to Nanz and Lanz type resolution:
- Type declarations already worked (MIR2 had them)
- Added to `as` cast expressions and function-style casts `i32(expr)`
- Enables eZ80/Agon Light 2 (24-bit native) and MZV VM (32-bit math)

### 7. Local Array Literals

```nanz
let arr: [u8; 5] = [10, 20, 30, 40, 50]
```

On Z80, local arrays can't live on the stack. The parser generates a mangled global `__arr_N` with the literal data and binds the local variable to its address. Same semantics as global arrays, scoped name.

### 8. else-if Chains

```nanz
if x == 0 {
    ...
} else if x == 1 {   // was parse error before
    ...
} else {
    ...
}
```

Desugars to nested `IfStmt` blocks. Unlocked Conway's Game of Life and other MinZ programs with chained conditionals.

## VIR Merge (parallel work by neighbor)

`feat/unified-vir-solver` merged to master during this session:
- Z3 joint isel+regalloc
- Z3-PFCCO (optimal calling conventions)
- CFG-aware solver
- 55 Z80-verified asserts, 496/496 coverage
- Available via `pipeline.Options{UseVIR: true}`

We fixed one merge conflict (duplicate frill import + conflict markers in `main.go`).

## Investigation: `x op (x + N)` Variable Reuse

Session briefing reported a panic in HIR lowerer for `key xor (key + 37)`. Investigation showed:
- **MIR2 VM**: works correctly (SSA handles it)
- **Z80 codegen**: produces wrong results (0 instead of 237 for u16 xor)
- Root cause: Z80 register allocator, not HIR lowerer (ADR-0006/0007)
- Tests added confirming correct MIR2 behavior

## Remaining MinZ Corpus Gaps (61/119 files)

| Category | Count | Notes |
|----------|-------|-------|
| Imports (search path) | 10 | Not a parser issue |
| `@inline` attribute | 5 | Could add as hint |
| `@asm` function attr | 4 | Use `asm z80 {}` inside fun instead |
| `module` keyword | 4 | Rejected — filename = module name |
| `@abi` attribute | 3 | Legacy |
| `impl` (done!) | 3 -> 0 | Fixed this session |
| `*expr` deref in exprs | 8 | Nanz uses `^expr` |
| `i32` cast (done!) | 7 -> 1 | Fixed, 1 remaining (`as` after `)`) |
| Other | ~20 | Misc legacy syntax |

## Commits (our 12)

```
b6dfc6b8 fix: resolve merge conflict in main.go (VIR merge artifact)
b02d2069 feat(stdlib+lanz+lizp): Stream abstraction + lambda/let-in/match
066995d3 feat(nanz): MinZ syntax compatibility — semicolons, let mut, else if
d06019aa feat(nanz): local array literals via let + else-if chains
de6d827e refactor(examples): *u8 → ^u8 pointer syntax across MinZ corpus
99795d2f feat(nanz+lanz): u24/i24/u32/i32 types in casts
07749727 refactor(examples): let mut → var, remove trailing semicolons
4244442f fix(nanz): remove semicolon tolerance — strict no-semicolons policy
79ef7d33 refactor(examples): final semicolon purge
0ce6556b feat(nanz): impl blocks — trait implementation with UFCS desugaring
```

## Test Results

| Package | Status |
|---------|--------|
| `pkg/lanz` | PASS (all tests including new lambda/let-in/match) |
| `pkg/lizp` | PASS (all tests including new fn/let*/case) |
| `pkg/hir` | PASS (including varreuse tests) |
| `pkg/pipeline` | PASS |
| `pkg/mir2` | PASS |
| `pkg/frill` | PASS |
| `pkg/nanz` | 9 pre-existing failures (ABI, imports) — no regressions |

## Next Session Candidates

1. **TinyFrill lexer** — self-hosting phase 1 (lexer for Frill written in Nanz)
2. **MinZ corpus: `@inline`** — add as compiler hint
3. **Stream: platform backends** — stdout via BDOS/MOS, file I/O
4. **Iterator chains in Nanz** — verify E2E parity with MinZ
5. **`impl` without trait** — `impl Type { ... }` for plain method grouping
