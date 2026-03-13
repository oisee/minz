# Proposal: MIR2-Level Function Inlining + Destructuring Elimination

**Status:** Draft
**Scope:** MIR2 pipeline, ~150-200 LOC new code
**Depends on:** None (independent of Phase 6e PBQP work)
**Discovered during:** PFCCO paper validation (swap degenerate case)

---

## Motivation: The Swap Problem

PFCCO can reduce `swap(a, b)` to a bare `RET` by reversing the calling
convention. This is locally optimal but globally wasteful:

```nanz
fun swap(a: u16, b: u16) -> (u16, u16) { return (b, a) }

fun main() -> void {
    let a: u16 = 1000
    let b: u16 = 2000
    print_val(a)
    let (a, b) = swap(a, b)   // swap + take second element = identity!
    print_val(b)               // b-after-swap == a-before-swap
}
```

**Current compiler output:**
```z80
main:
    LD HL, 1000
    CALL print_val        ; print a
    LD DE, 2000
    EX DE, HL             ; adapter for swap's reversed convention
    CALL swap             ; RET — 27T wasted
    LD H, D               ; extract second return value
    LD L, E
    JP print_val          ; print b (which is old a = 1000)
```

**Optimal output (with inlining + copy propagation):**
```z80
main:
    LD HL, 1000
    CALL print_val        ; print a (HL=1000)
    JP print_val          ; print b = old a = still 1000!
```

The entire swap, its adapter moves, and the result extraction disappear.
The compiler just needs to see that `swap(a,b).1 == a`.

---

## What Exists Today

### Inlining infrastructure (NOT wired to MIR2 pipeline)

| Component | Location | Status |
|-----------|----------|--------|
| IR-level InliningPass | `pkg/optimizer/inlining.go` (299 LOC) | Works for old IR, not MIR2 |
| SimpleInliningPass | `pkg/optimizer/semantic_passes.go` | AST-level, before IR |
| StrategyInline heuristic | `pkg/optimizer/calling_convention_heuristics.go` | Decision logic only |
| Iterator callback inlining | `pkg/hir/lower.go:2088-2209` | HIR-level, forEach/fold only |
| EQU collapse | `pkg/mir2/z80codegen.go:158-269` | Post-RA, single-JP only |

### Optimization passes (wired to MIR2 pipeline)

| Pass | Location | Relevant? |
|------|----------|-----------|
| PropagateConstants | `pkg/mir2/constprop.go` | Yes — handles constant block params |
| FoldConstants | `pkg/mir2/constprop.go` | Yes — evaluates constant expressions |
| DeadStoreElim | `pkg/mir2/dse.go` | Yes — removes unused pure ops |
| EliminateDeadBlocks | `pkg/mir2/` | Yes — removes unreachable code |
| ConstantCallElim | `pkg/mir2/constprop.go` | Yes — eliminates calls with known results |

### Key gap

**No MIR2-level function inlining.** The constant folding pipeline can
eliminate calls when all arguments are constants (ConstantCallElim), but
it cannot inline a function with dynamic arguments. The swap case requires
inlining to expose the copy for propagation.

---

## Proposed Design

### Phase 1: Trivial Function Inlining (~100 LOC)

New file: `pkg/mir2/inline.go`

```go
// InlineTrivial replaces CALL instructions to trivial functions
// with the function body, substituting arguments for parameters.
//
// A function is "trivial" if:
//   - Body has exactly 1 block (entry) with ≤ N instructions (N=4)
//   - No loops (single block guarantees this)
//   - No calls to other functions (leaf only)
//   - Not @extern, not main, not an interrupt handler
//   - Not recursive
//
// Multi-return is handled: each OpCall result register maps to the
// corresponding return register in the inlined body.

func InlineTrivial(mod *Module, maxSize int) bool {
    changed := false
    candidates := findTrivialFunctions(mod, maxSize)

    for _, f := range mod.Funcs {
        for _, b := range f.Blocks {
            for i, inst := range b.Instrs {
                if inst.Op != OpCall { continue }
                callee := mod.FuncByName(inst.Aux.(string))
                if !candidates[callee] { continue }

                // Replace OpCall with inlined body:
                // 1. Map callee params → caller args
                // 2. Map callee returns → call result regs
                // 3. Splice instructions (substituting registers)
                // 4. Remove the CALL instruction
                inlineCallSite(f, b, i, callee)
                changed = true
            }
        }
    }
    return changed
}
```

**Swap example after inlining:**
```
Before:  %3, %4 = CALL swap(%1, %2)    // %1=a, %2=b
After:   %3 = Move %2                   // first return = b
         %4 = Move %1                   // second return = a
```

### Phase 2: Copy Propagation Through Moves (~50 LOC)

Extend existing `PropagateConstants` or add `PropagateCopies`:

```go
// PropagateCopies replaces uses of OpMove destinations with their sources.
// This eliminates the copies left behind by inlining.
//
// %3 = Move %2     →  (delete, replace all uses of %3 with %2)
// %4 = Move %1     →  (delete, replace all uses of %4 with %1)

func PropagateCopies(f *Func) bool {
    changed := false
    for _, b := range f.Blocks {
        for _, inst := range b.Instrs {
            if inst.Op == OpMove && !inst.Dst.IsPhys() {
                replaceAllUses(f, inst.Dst, inst.Args[0])
                changed = true
            }
        }
    }
    // DeadStoreElim will clean up the now-unused Moves
    return changed
}
```

**After copy propagation:**
```
Before:  %3 = Move %2;  use(%3)   →   use(%2)
         %4 = Move %1;  use(%4)   →   use(%1)
```

The `let (a, b) = swap(a, b); print_val(b)` becomes `print_val(a)`.

### Phase 3: Pipeline Integration (~10 LOC)

In `pkg/pipeline/pipeline.go`, add after constant folding, before PFCCO:

```go
// Inline trivial functions (exposes copies for propagation)
InlineTrivial(mod, 4)

// Run copy propagation + DSE to clean up
for _, f := range mod.Funcs {
    PropagateCopies(f)
    DeadStoreElim(f)
}
```

**Important:** Inlining must run BEFORE PFCCO, because:
- Inlining removes calls from the call graph
- PFCCO then has fewer edges to optimize
- Trivial functions that would be degenerate (bare RET) are eliminated

---

## Syntax Decision: Tuple Indexing

### Survey of other languages

| Language | Multi-return model | Indexing syntax | Destructuring |
|----------|-------------------|----------------|---------------|
| Rust | Tuple value | `.0`, `.1` | `let (a, b) = foo()` |
| Swift | Tuple value | `.0`, `.name` | `let (a, b) = foo()` |
| Zig | Anon struct | `[0]`, `[1]` | `const a, const b = foo()` |
| Odin | Polyadic | None | `x, y := foo()` |
| Go | Polyadic | None | `x, y := foo()` |
| **Nanz** | **Polyadic** | **None (proposed: `.0`)** | `let (x, y) = foo()` |

### Recommendation: Don't add `.0`/`.1` syntax (yet)

**Reasons:**
1. Nanz's multi-return is polyadic (values in separate registers), not
   an aggregate. Adding indexing syntax implies a tuple *value* that can
   be stored, passed, and indexed — which doesn't exist.
2. Destructuring `let (_, b) = foo()` already handles the common case.
3. On Z80 with 3 register pairs max, you'll never have more than 3 return
   values. Indexing adds complexity for minimal benefit.
4. Odin and Go made the same choice and it works well.

**If we add it later:** Use `.0`/`.1` (Rust style) — clear, widely understood,
no ambiguity with field access (field names can't start with digits).

---

## The "In-Mind Swap" Concept

A fascinating property of PFCCO + inlining:

```
Before swap:  reg_A = var_a(5),  reg_B = var_b(6)
[inline swap → just copy-prop]
After swap:   reg_A = var_b(5),  reg_B = var_a(6)  ← tags changed, no instructions
```

The swap happens entirely in the compiler's symbol table. No instructions
emitted, zero T-states, zero bytes. This is the most aggressive form of
**live range renaming** — the function's semantics dissolve into SSA
register assignments.

This works whenever:
- The function body is pure register rearrangement (swap, rotate, project)
- Inlining exposes it as Move chains
- Copy propagation eliminates the Moves
- The subsequent code's calling conventions happen to align

When conventions don't align (next function needs the value in a different
register), the physical move reappears — but only the *necessary* one,
not the full swap.

---

## Cost-Benefit Analysis

### Costs
- ~150-200 LOC new code (inline.go + copy prop extension)
- Must handle edge cases: recursive calls (don't inline), side effects
  (preserve), multi-return register mapping
- Small compile-time increase (one extra pass over call sites)

### Benefits
- Eliminates degenerate bare-RET functions (the swap problem)
- Reduces CALL+RET overhead for trivial functions (27T savings per site)
- Enables "in-mind swap" — zero-cost register renaming
- Simplifies PFCCO's job (fewer edges in call graph)
- The existing EQU collapse handles single-JP; this handles the rest

### What it does NOT solve
- Non-trivial functions with loops or calls (by design — inline only leaves)
- The multi-caller cost accounting issue (PFCCO still uses unit weights)
- Peephole gaps in the register allocator (BUG-001 parallel-copy bloat)

---

## Implementation Plan

| Step | LOC | Depends on | Priority |
|------|-----|-----------|----------|
| 1. `InlineTrivial()` | ~100 | None | High |
| 2. `PropagateCopies()` | ~50 | Step 1 | High |
| 3. Pipeline wiring | ~10 | Steps 1-2 | High |
| 4. Tests (swap, identity, project) | ~80 | Steps 1-3 | High |
| 5. Tuple `.0`/`.1` syntax | ~40 parser | Independent | Low (defer) |

Total: ~240 LOC for steps 1-4. Can be done in one session.

---

## Open Questions

1. **Inline threshold:** 4 instructions? 8? Should it be T-state-based
   (inline if body cost < CALL+RET overhead of 27T)?
2. **Multi-return inlining order:** Should we inline before or after
   PFCCO? Before seems right (fewer edges), but after could use contract
   info to make better decisions.
3. **Interaction with SMC:** Functions with `OpPatch`/`OpPatchSlot` should
   NOT be inlined (SMC anchors need stable addresses).
4. **Interaction with iterator fusion:** Already inlined at HIR level.
   MIR2 inlining is for non-iterator trivial functions only.

---

## References

- gingerBill, "Multiple Return Values Research" (Odin design rationale)
- Rust RFC 184 — Tuple Accessors (`.0`/`.1` syntax)
- LLVM SROA pass — Scalar Replacement of Aggregates
- Rust MIR SROA PR #102570
