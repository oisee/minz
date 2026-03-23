# Session Report: Frill Language + eZ80 Backend + Z3 Allocator

**Date:** 2026-03-22 — 2026-03-23
**Branch:** master (all work committed directly)
**Commits:** 78

---

## What Was Built

### 1. Frill Language — ML-style Functional Frontend (38 features)

**From zero to 38 features in one session.** Frill is an ML-style functional
language that compiles to Z80 through the MinZ HIR→MIR2→LIR pipeline.

| Category | Features |
|----------|----------|
| Core | functions, type inference, if/then/else, arithmetic, comparisons |
| Functional | pipe `\|>`, compose `>>`, lambda, currying, let-in, where |
| Imperative | mutable vars `<-`, while, for-range, do notation |
| Types | ADT, pattern match + guards + exhaustive check, tuples, records, type classes |
| Memory | peek (read), poke (write), string literals |
| Verification | assert, property testing (256 exhaustive per prop), QTT linearity |
| Modularity | import .frl/.nanz, extern, inline asm, cross-language calls |

**Key achievement:** QTT (Quantitative Type Theory) annotations — `!` (linear: use once)
and `~` (erased: never use) enforced at compile time. First QTT implementation
targeting Z80.

**Parser:** 2031 LOC (`pkg/frill/frill.go`), 13 unit tests.

### 2. eZ80 Backend — Agon Light 2 Support

- `TargetInfo` struct + `pkg/ez80/` independent package
- `EZ80CostTable` — PBQP costs for eZ80 (MLT 6T, LEA 3T, stack 8T)
- Native MIR2→eZ80 codegen (919 LOC) — MLT, LEA, 24-bit immediates
- MZA: `.ASSUME ADL=1`, `DW24`/`DL` directives, 24-bit encoding
- Pipeline: `minzc program.nanz -b ez80` → Agon MOS binary
- **Verified on fab-agon-emulator:** "Hello from MinZ on Agon Light 2!" printed

### 3. Z3 SMT-Based Optimal Register Allocation

- `pkg/z3alloc/` — generates SMT-LIB2, calls Z3 subprocess, parses model
- Formulation: variables = phys locs, constraints = interference, objective = min cost
- Finds provably optimal allocation in 30ms for small functions
- 2 tests: `add(a,b)` → {A,B,A} cost=0; triple with interference → {A,E,L,A}
- Falls back to PBQP when Z3 unavailable

### 4. LIR Meta-Instructions

- `MetaInst` with 5 kinds: Pin, SpillBarrier, CostOverride, FuseNext, Comment
- WFC `NewWFCState` collects MetaPin constraints and narrows LocSets before propagation
- Foundation for solver-guided optimization without backtracking

### 5. Codegen Improvements

- `CALL+RET → JP` peephole — saves 17 T-states per tail call site
- ADR-0038: TSMC spill tiers (L4/L5) — self-modifying code as register preservation
- ADR-0037 updated with TSMC costs and eZ80 comparisons
- Honest code quality assessment with SDCC comparison in Frill guide

---

## Artifacts

| Artifact | Size | Description |
|----------|------|-------------|
| `pkg/frill/frill.go` | 2031 LOC | Frill parser + type checker |
| `pkg/frill/frill_test.go` | 300 LOC | 13 unit tests |
| `pkg/ez80/` | 2475 LOC | eZ80 backend (codegen + cost + tests) |
| `pkg/z3alloc/` | 363 LOC | Z3 optimal allocator |
| `stdlib/frill/` | 221 LOC | math (26 funcs), functional, tokenizer, I/O |
| `examples/frill/` | 556 LOC | 9 demos (basics, math, game, showcase, calculator, etc.) |
| `docs/Frill_Language_Guide.md` | 834 LOC | Complete guide with assembly examples |
| `docs/book/` | EPUB 21K, PDF 68K | Rendered book |
| `docs/adr/0036-0038` | 1186 LOC | 3 ADRs (eZ80, target split, TSMC spill) |
| `stdlib/agon/tui_render.nanz` | 223 LOC | Agon TUI library |

**Total new code:** ~7500 LOC across 30+ files.

---

## What Works

- Frill compiles all 9 demos through full pipeline
- 1000+ property-based test checks pass at compile time
- `fib(7) = 13` prints correctly on CP/M emulator (720 bytes)
- "Hello from MinZ on Agon Light 2!" on Agon CLI emulator
- Z3 allocator finds optimal register assignment
- All existing MinZ tests pass (no regressions)

## Known Issues

| Issue | Impact | Root Cause |
|-------|--------|-----------|
| MIR2 VM: peek at string offset > 0 | Blocks tokenizer testing | VM string pointer model |
| MIR2 VM: 3-param arg clobber | Blocks ADT payload, apply(op,a,b) | Register save across args |
| MIR2 optimizer: `if x == 0 then A else B` folds wrong | Workaround: use `x > 0` | CondRetSink/BranchEquiv |
| LIR: asm blocks empty | Use `--lir=false` for asm | LIR doesn't pass through OpAsm |
| eZ80 codegen: cross-module label mismatch | Some imports fail | inst.Sym vs callee.Name |

---

## Metrics

| Metric | Value |
|--------|-------|
| Session commits | 78 |
| New LOC | ~7500 |
| Frill features | 38 |
| Frill demos | 9 |
| Frill stdlib functions | 26 (math) + 15 (functional) + 20 (tokenizer) + 5 (I/O) |
| Compile-time assertions | 100+ explicit + 1000+ property checks |
| ADRs written | 2 new (0038 TSMC, 0037 update) |
| Backends touched | Z80, eZ80, Z3, LIR meta |
| Guide pages | 834 lines, EPUB + PDF |

---

## Next Session Briefing

### Priority A: Fix MIR2 VM (unblocks everything)

Three bugs in `pkg/mir2/vm.go` block Frill's self-hosting path:

1. **String pointer arithmetic** — `peek(str + N)` where N > 0 fails with "index out of range".
   The VM stores strings as indexed objects, not raw memory. Fix: map string symbols to
   contiguous memory in VM heap, allow pointer arithmetic on them.

2. **3-param arg clobber** — `f(a, b, c)` where first param gets overwritten by third.
   The VM or HIR lowering assigns params to registers without saving. Fix: insert explicit
   copies in HIR→MIR2 lowering for 3+ param functions, or fix VM arg passing order.

3. **`== 0` comparison fold** — `if x == 0 then A else B` gets constant-folded to always B.
   CondRetSink or BranchEquiv incorrectly eliminates the zero branch. Fix: check the
   optimization pass that handles `cmp.eq %r, 0` patterns.

Fixing these three unblocks: ADT payload match, tokenizer string traversal, 3-param
apply functions, and the entire self-hosting parser.

### Priority B: Frill → Nanz Transpiler

If VM fixes are complex, an alternative path: Frill source → Nanz source → compile.
The transpiler would be ~300 LOC, translating Frill syntax to equivalent Nanz.
This bypasses VM issues entirely and gives Frill programs access to full Nanz stdlib.

### Priority C: More Frill Features

- **Parametric polymorphism** — `let id (x : 'a) = x` (needs u16 VM fix first)
- **Effect system** — `IO u8` vs pure `u8`
- **Frill REPL** — interactive via MZV
- **Frill → JavaScript** — browser playground

### Files to Start With

```
pkg/mir2/vm.go              — VM string/arg bugs
pkg/mir2/z80codegen.go      — CondRetSink fold bug
pkg/frill/frill.go          — Frill parser (add features)
stdlib/frill/tokenizer.frl  — self-hosting tokenizer
```
