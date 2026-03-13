# Note: Swap Degenerate Case & Multi-Caller Cost Accounting

**Date:** 2026-03-13
**Status:** Open — to revisit after Phase 6e (PBQP edge costs)

## The Issue

PFCCO optimizes swap() to a bare `RET` by reversing the calling convention:
`swap(a=DE, b=HL) -> (HL, DE)` — values arrive already "swapped."

This is correct for 0-1 callers but **suboptimal for N>1 callers:**

| Callers | Convention A (EX inside) | Convention B (bare RET) |
|---------|------------------------|------------------------|
| 0 | 14T (dead code) | 10T (dead code) |
| 1 | 14T + 17T = 31T | 10T + 21T = 31T |
| 2 | 14T + 34T = 48T | 10T + 42T = 52T |
| 3 | 14T + 51T = 65T | 10T + 63T = 73T |

Breakeven at 1 caller. Beyond that, keeping EX inside swap wins.

## Root Cause

The PFCCO cost function evaluates adapter cost + transfer cost but currently
uses **unit edge weights**. It doesn't multiply by call count. The topological
DP picks the locally cheapest convention (bare RET = 0 adapter cost) without
accounting for the N × EX-at-call-site overhead.

## Conceptual Insight: "In-Mind Swap" / Register Renaming

The bare-RET swap is conceptually a **register rename** — the swap happens
in the compiler's symbol table, not in the CPU:

```
Before swap:  reg_A = var_a(5),  reg_B = var_b(6)
CALL swap:    RET — no registers change
After swap:   reg_A = var_b(5),  reg_B = var_a(6)  ← tags changed
```

With inlining, the CALL+RET pair (27T) would disappear entirely, making
this a true zero-cost operation. Without inlining, it wastes 27T per call.

## Resolution Options

1. **Edge-weight-aware PFCCO** — multiply transfer costs by call count
   (already in §8 Future Work as "profile-guided weighting")
2. **Inlining pass** — detect trivial function bodies (RET, single instruction)
   and inline them, eliminating CALL+RET overhead
3. **Post-PFCCO check** — if function body is bare RET, force inlining
4. **Phase 6e (PBQP edge costs)** may naturally address this through
   binary cost matrices that capture caller/callee interaction

## Decision

Defer until after Phase 6e. If PBQP edge costs solve it, great.
If not, add special-case handling for trivial function bodies.
