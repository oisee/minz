# Session Report 107: Frill Language Maturity

**Date:** 2026-03-23
**Branch:** master (7 commits, all pushed)
**Previous:** Report session-frill-ez80-z3 (2026-03-22/23)

---

## What Was Done

### Bug Fixes (Priority A from previous session)

All three blocking bugs resolved:

| Bug | Root Cause | Fix |
|-----|-----------|-----|
| String pointer arithmetic | Assert parser didn't accept string args; VM didn't cache string allocations; Frill parser duplicated string indices | String args in asserts + VM `globalSyms` caching + `internString()` dedup |
| "3-param arg clobber" | `BranchEquiv` tested only `[const, 0, 0]` boundary inputs — missed interactions | Sweep each other param (256 values) + cross-product of representatives `{0, 1, 2, 127, 255}` |
| `== 0` comparison fold | Same `BranchEquiv` bug — single boundary point couldn't distinguish then/else for multi-param functions | Same fix as above |

### New Language Features

| Feature | Description | LOC |
|---------|-------------|-----|
| **Parametric polymorphism** | `let id (x : 'a) : 'a = x` — type variables monomorphized at call sites (Rust-style, zero overhead) | ~180 |
| **Effect system** | `IO` return type annotation; pure functions cannot call IO — compile error with helpful message | ~130 |
| **LetInExpr** | `let x = expr in body` works inside if/else branches (was TODO stub that discarded bindings) | ~40 |
| **u16 type tracking** | `varTypes` map tracks declared parameter/let-binding types; VarRefExpr uses declared type instead of defaulting to u8 | ~15 |
| **\x hex escapes** | `\xNN` in string literals (was missing) | ~15 |

### Demos

| Demo | Description | Asserts |
|------|-------------|---------|
| `expr_eval.frl` | Recursive descent expression evaluator: +, -, *, /, parens, integers. Correct precedence. | 15 |
| `stress_test.frl` | 11 categories: polymorphism, LetInExpr, u16, strings, tokenizer, dispatch, loops, recursion (Ackermann, Collatz), pipes, hex escapes, properties | 91 + 3 props |
| `music.frl` | Music DSL: 13-variant Note ADT, frequency tables, melody encoding/decoding, transpose, musical invariants | 33 + 2 props |

---

## Metrics

| Metric | Before | After |
|--------|--------|-------|
| Frill features | 38 | 41 (+polymorphism, effects, LetInExpr) |
| Frill examples | 9 | 12 (+expr_eval, stress_test, music) |
| Compile-time checks | ~1500 | **3351** (279 asserts + 12 props x 256) |
| frill.go | 2031 LOC | **2436 LOC** (+405) |
| Frill parser tests | 13 | 13 (unchanged — coverage via compile-time asserts) |

### All Examples Pass

```
basics.frl:          17 functions
calculator.frl:      36 functions
expr_eval.frl:       31 functions
functional_demo.frl: 49 functions
game.frl:             7 functions
hello_cpm.frl:       31 functions
interactive.frl:     31 functions
math.frl:            20 functions
showcase.frl:        41 functions
stdlib_demo.frl:     30 functions
stress_test.frl:     55 functions
music.frl:           26 functions
```

12/12 pass, 0 regressions. Go test suite: mir2, frill, hir, pipeline all green.

---

## Commits

```
48fde73d feat(frill): Music DSL — declarative melodies on Z80
f5fe6a53 test(frill): stress test — 91 asserts + 3 props (859 checks)
1f6a55d4 feat(frill): effect system — IO vs pure function tracking
1100506b feat(frill): parametric polymorphism with monomorphization
8246ccba feat(hir): LetInExpr — let-in works inside if/else branches
0575741c feat(frill): expression evaluator + u16 type tracking + tokenizer u16
40ffa1d6 fix(frill+mir2): string assert args, BranchEquiv sweep, \x escapes
```

---

## Known Issues

| Issue | Impact | Workaround |
|-------|--------|-----------|
| `let ... in` inside `parseBodyExpr` still desugars to VarDeclStmt (not LetInExpr) | None — both paths work | N/A |
| `>>` composition generates duplicate labels with `let name = f >> g` | Assembler error | Use different name or test in separate file |
| Assert parser only accepts int/string/constructor args, not expressions | Can't test `f(g(x))` directly | Wrap in named function |
| `poke` in function body has parsing issues | Can't test poke in stress test | Use `do poke` form |

## Next Session Briefing

### Ready to Build

1. **State Machine DSL** — ADT for states, pattern match for transitions. Traffic light, parser states.
2. **Pixel Art DSL** — Pattern matching for sprite generation. MZV canvas visualization.
3. **TUI DSL** — Declarative screen layout via pipe chains.
4. **Frill REPL** — Interactive evaluation via MZV.
5. **Book chapter** — "Frill: ML on Z80" with assembly examples.

### Frill Feature Gaps (for completeness)

- **Lists/arrays** as first-class values (currently only peek/poke)
- **String type** (currently u16 pointers)
- **Modules/namespaces** (currently flat namespace with imports)
- **Multi-line match arms** (currently single-expression per arm)
- **Recursive ADTs** (e.g., `type List = Nil | Cons of u8`)

### Files to Start With

```
examples/frill/       — add new demos here
pkg/frill/frill.go    — parser + type checker (2436 LOC)
stdlib/frill/          — standard library modules
docs/Frill_Language_Guide.md — update with new features
```
