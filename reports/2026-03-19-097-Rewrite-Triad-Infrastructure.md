# Report 097 — Rewrite Triad: Infrastructure + Pipeline Integration

**Date:** 2026-03-19
**Status:** ✅ Wired into pipeline, 94.1% convergence with Go originals
**LOC:** ~4200 new (library + runner + tests + rules + adapter)

---

## What Was Built

Three declarative optimization engines in `pkg/rewrite/`, operating on
abstract IR interfaces (`IRGraph`, `IRGraphMut`) with zero imports from
HIR/MIR2/LIR:

### 1. Shared S-expr Parser (`pkg/rewrite/sexpr/`)
- 157 LOC, 6 tests
- Extracted from `pkg/lir/isle/sexpr.go`
- `pkg/lir/isle/sexpr.go` now delegates to this (backward-compatible wrapper)
- All existing LIR ISLE tests pass unchanged

### 2. Expanded ISLE Engine (`pkg/rewrite/isle/`)
- 542 LOC, 7 tests
- Full term rewriting: pattern matching, priority-ordered rules, fixpoint
- **New: Guard clauses** — `(rule (add ?x (const ?n)) (if (< ?n 256)) (addi ?x ?n))`
- **New: Extern constructors** — Go callbacks: `registry.Register("is_pure", func(...))`
- Both flat `(if < ?n 256)` and nested `(if (< ?n 256))` guard syntax

### 3. Grace Graph Pattern Engine (`pkg/rewrite/grace/`)
- 1255 LOC, 13 tests (7 Cypher-equivalent categories + 6 compiler patterns)
- S-expr syntax: `(grace rule-name priority (match ...) (where ...) (action ...))`
- Subgraph matching on abstract IRGraph (1-N block variables, backtracking)
- Built-in predicates: `pred-count`, `all-pure`, `no-params`, `inst-count`,
  `param-count`, `dominates`, `term-kind`, `use-count`
- Custom predicate/action registry (Go escape hatches for complex mutations)
- Edge kinds: `succ`, `then`, `else`, `back` (layout-aware back-edge detection)

### 4. Datalog Fact Database (`pkg/rewrite/datalog/`)
- 101 LOC, 2 tests
- Extracted from `pkg/lir/rules.go` (the `FactDB` type)
- S-expr parser: `(fact reg "A" "8" "acc")`
- Wildcard queries: `db.Query("reg", "_", "8", "_")` → all 8-bit registers

### 5. MIR2 Graph Adapter (`pkg/mir2/graph_adapter.go`)
- 210 LOC, 3 tests
- `FuncAsGraph(f)` → read-only `IRGraph`
- `FuncAsMutGraph(f)` → mutable `IRGraphMut` with hoist, remove, set-term
- Tests prove Grace rules match against real MIR2 `Func` structures

### 6. Declarative Rule Files (`rules/`)
- 147 LOC across 6 files
- `mir2_simplify.isle` — 18 algebraic identity rules (add x 0 → x, neg neg x → x, inc/dec)
- `mir2_dse.grace` — dead store elimination spec
- `mir2_condret.grace` — conditional return sinking spec
- `mir2_deadarg.grace` — dead block argument elimination spec
- `mir2_absdiff.grace` — abs-diff fusion spec
- `mir2_joinret.grace` — join-ret splitting spec

---

## What Grace Can Express (Cypher Equivalence)

Grace was tested against 7 categories of graph queries, proving equivalent
expressiveness to Cypher on compiler IR:

| # | Category | Grace Query | Cypher Equivalent |
|---|----------|-------------|-------------------|
| 1 | Block discovery | `(block ?b (term-kind "jump"))` | `MATCH (b:Block) WHERE b.term='jump'` |
| 2 | Edge patterns | `(edge ?a ?b "then")` | `MATCH (a)-[:THEN]->(b)` |
| 3 | Control flow | `(edge ?a ?b "succ")` | `MATCH (a)-[:SUCC]->(b)` |
| 4 | Diamond detect | 4 blocks + 4 edges | `MATCH (c)-[:THEN]->(t), (c)-[:ELSE]->(e), (t)->(m)<-(e)` |
| 5 | Dominance | `(dominates ?a ?b)` | `MATCH (a)-[:DOMINATES]->(b)` |
| 6 | Inst patterns | `(all-pure ?b)` | `WHERE all(i IN b.insts WHERE i.pure)` |
| 7 | Aggregation | `(pred-count ?b == 1)` | `WHERE count(predecessors) = 1` |

All 7 categories tested and passing on mock IR graphs.

---

## Pipeline Integration (Phase 2)

### Wiring

Grace rules are now callable from the compilation pipeline via
`Options{UseGrace: true}`. The `grace_runner.go` file (370 LOC) provides:

- **5 embedded Grace rules** with correct priority ordering (DSE=50 > DeadBlockArg=45 > SplitJoinRet=30 > CondRetSink=20 > FuseAbsDiff=10)
- **5 custom predicates**: `no-carry-clobber`, `is-ret-of-param`, `has-dead-params`, `has-dead-insts`, `has-abs-diff-pattern`
- **5 custom actions**: `condRetSink`, `splitJoinRet`, `elimDeadParams`, `elimDeadInsts`, `fuseAbsDiff`
- **Statistics tracking**: `GraceStats` counts per-rule applications across the module
- **`RunGracePasses(f, stats)`**: single entry point running all rules to fixpoint

### Convergence Results (Nanz Corpus)

```
=== Grace Verification ===
Files: 17, Pass: 17, Fail: 0        ← 100% structural correctness
Rate: 100.0%

=== Grace Convergence ===
Files: 17, Matched: 16, Diverged: 1  ← 94.1% byte-identical assembly
Rate: 94.1%

Grace stats: 89 funcs, 63 rewrites
  dead-store-elim = 34
  cond-ret-sink   = 14
  dead-block-arg  = 11
  split-join-ret  = 4
```

**17/17 pass MIR2 verification** — no structural corruption.
**16/17 produce byte-identical assembly** — the Grace path replicates Go behavior.

### The 1 Divergence

`08_arena_allocator.nanz` — register allocation difference (`LD HL, 51456`
vs `LD DE, 51456`). Semantically equivalent, different register choice.
Root cause: Grace runs all rules as a single fixpoint vs Go's strict
sequential ordering. The arena allocator's complex CFG exposes an ordering
sensitivity in the register allocator's PBQP cost model.

### Pipeline.go Changes

```go
if opt.UseGrace {
    mir2.RunGracePasses(f, opt.GraceStats)  // all 5 rules to fixpoint
    mir2.EliminateDeadBlocks(f)
    if mir2.BranchEquiv(m, f) {             // stays Go (needs VM/solver)
        mir2.EliminateDeadBlocks(f)
        mir2.RunGracePasses(f, opt.GraceStats)
        mir2.EliminateDeadBlocks(f)
    }
    for _, blk := range f.Blocks {
        mir2.FusionSubCmpInBlock(blk)       // unconditional, same as Go path
    }
} else {
    // original Go path (unchanged)
}
```

---

## What Is NOT Done (Honest Assessment)

### ❌ Not Wired Into Compilation Pipeline

The Grace rules are **parsed and matched in tests only**. During actual
compilation (`pkg/pipeline/pipeline.go`), all 5 optimization passes run as
pure Go code:

```
mir2.DeadStoreElim(f)       ← Go, not Grace
mir2.DeadBlockArgElim(f)    ← Go, not Grace
mir2.SplitJoinRet(f)        ← Go, not Grace
mir2.CondRetSink(f)         ← Go, not Grace
mir2.FuseAbsDiff(f)         ← Go, not Grace
```

### ❌ No `--legacy-opts` Flag

The plan called for keeping Go originals behind `--legacy-opts` during
transition. This flag was not implemented because the transition itself
hasn't started — there's no Grace-based replacement running in production.

### ❌ No Z80 Reject/Backtrack

Grace operates on CFG patterns (block shapes, edge topology, instruction
purity). It does NOT handle Z80-specific constraints, register allocation,
or instruction encoding validation. Those are handled by:

- **WFC** (Wave Function Collapse) in the LIR backend — constraint
  propagation with backtracking on register assignment
- **Z80 descriptor** (`pkg/lir/z80.go`) — encoding rules, DD/FD prefix
  forbidden combinations
- **Peephole** (`pkg/lir/z80peephole.go`) — post-allocation cleanup

Grace is a **pre-regalloc CFG optimizer**, not a backend constraint solver.

### ❌ Grace Rules Don't Generate Tighter Code (Yet)

Since Grace rules aren't running, there's no code density improvement.
The existing Go implementations already produce good code. The value
proposition is **maintainability** (declarative rules vs hand-coded Go),
not better output.

### ⚠️ pkg/rewrite/isle/ vs pkg/lir/isle/

Two separate ISLE implementations exist:

- `pkg/rewrite/isle/` — the new expanded version (guards, externs)
- `pkg/lir/isle/` — the original, used by LIR combine.go (isel pipeline)

They share the S-expr parser (`pkg/rewrite/sexpr/`) but are otherwise
independent. The LIR backend has NOT been migrated to use `pkg/rewrite/isle/`.

---

## What Would Be Needed to Complete the Migration

1. **Wire Grace runner into pipeline** (~50 LOC in `pipeline.go`)
   - Load `.grace` files (or embed them)
   - Replace Go function calls with `grace.ApplyRules(FuncAsMutGraph(f), rules, preds, 100)`
   - Register custom predicates/actions for complex mutations

2. **Implement custom actions** (~300 LOC)
   - `condRetReplaceTerm` — the TermBrIf→TermCondRet conversion
   - `splitJoinRet` — the block cloning + edge patching
   - `fuseAbsDiff` — the Sub/Cmp rewriting
   - `elimDeadParams` — the param compaction + edge arg patching

3. **Verify byte-identical output** on 131/173 example corpus

4. **Migrate LIR's ISLE** to `pkg/rewrite/isle/` (~100 LOC adapter)

---

## Test Results

```
pkg/rewrite/sexpr     6/6 PASS
pkg/rewrite/isle      7/7 PASS
pkg/rewrite/grace    13/13 PASS
pkg/rewrite/datalog   2/2 PASS
pkg/lir/isle          6/6 PASS (0 regressions)
pkg/lir               all PASS (0 regressions)
pkg/mir2/adapter      3/3 PASS
pkg/pipeline          all PASS (0 regressions)
```

---

## File Map

```
pkg/rewrite/                          ← NEW (zero imports from hir/mir2/lir)
├── irgraph.go                        ← abstract interfaces (IRNode, IRBlock, IRGraph, IRGraphMut)
├── sexpr/sexpr.go                    ← shared S-expr parser
├── isle/                             ← expanded term rewriting
│   ├── isle.go                       ← core parser + matcher + rewriter
│   ├── guards.go                     ← (if ...) clause evaluation
│   └── externs.go                    ← Go callback registry
├── grace/                            ← graph pattern engine
│   ├── grace.go                      ← types + ApplyRules entry point
│   ├── parser.go                     ← S-expr → GraceRule
│   ├── matcher.go                    ← subgraph matching (1-N block vars)
│   ├── actions.go                    ← rewrite execution
│   ├── predicates.go                 ← built-in helpers + custom registry
│   └── grace_test.go                 ← 13 tests (7 Cypher categories)
├── datalog/                          ← fact database
│   ├── datalog.go                    ← FactDB with wildcard queries
│   └── parser.go                     ← S-expr → FactDB

pkg/mir2/
├── graph_adapter.go                  ← FuncAsGraph / FuncAsMutGraph adapters

pkg/lir/isle/
├── sexpr.go                          ← now delegates to pkg/rewrite/sexpr

rules/                                ← declarative rule specifications
├── mir2_simplify.isle                ← 18 algebraic identity rules
├── mir2_dse.grace                    ← dead store elimination
├── mir2_condret.grace                ← conditional return sinking
├── mir2_deadarg.grace                ← dead block arg elimination
├── mir2_absdiff.grace                ← abs-diff fusion
└── mir2_joinret.grace                ← join-ret splitting
```

---

## Verdict

**Infrastructure: ✅ Complete and tested.**
**Integration: ❌ Not started.**
**Value delivered: Foundation for future declarative optimization rules.**

The engines work. The interfaces are clean. The tests prove Grace can match
real MIR2 structures. But until someone wires `grace.ApplyRules()` into
`pipeline.go` and implements the custom action callbacks, the actual
compilation path is unchanged.

This is scaffolding, not a ship. Good scaffolding, but scaffolding.
