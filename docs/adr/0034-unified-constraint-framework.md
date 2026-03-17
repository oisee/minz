# ADR-0034: Unified Constraint Framework — ISLE+WFC+PBQP at Every Level

**Date:** 2026-03-17
**Status:** Proposal (informed by LIR backend experience)
**Supersedes:** None
**Related:** ADR-0030 (LIR), ADR-0033 (LIR pipeline)

## Context

The LIR backend (ADR-0033) proved that ISLE term-rewriting + WFC constraint propagation
work together for instruction selection and register allocation, achieving 94.6% corpus
coverage on the first iteration.

Observation: **every compiler optimization is a constraint satisfaction problem.** The
same techniques (pattern matching, constraint propagation, graph coloring) could replace
the hand-written Go passes at HIR and MIR2 levels.

Currently, MinZ has 11+ hand-written MIR2 optimization passes:
- PropagateConstants, FoldConstants, SimplifyIdentities (3 passes, iterated to fixpoint)
- DeadStoreElim, DeadBlockArgElim, EliminateDeadBlocks
- BranchEquiv, CondRetSink, SplitJoinRet, FuseAbsDiff
- InlineTrivial, OptimizeContracts, LUTGen

Each is imperative Go code. Each could be a set of declarative ISLE rules.

## The Three Solvers

### ISLE — "What transformation to apply"

Term-rewriting DSL. Matches a pattern, produces a replacement.
Works at any abstraction level — the terms just change.

```lisp
;; HIR level: algebraic simplification
(rule (add ?x (const 0))  ?x)
(rule (mul ?x (const 1))  ?x)
(rule (if (const true)  ?then ?else)  ?then)
(rule (if (const false) ?then ?else)  ?else)

;; MIR2 level: strength reduction
(rule (mul ?x (const 2))  (add ?x ?x))
(rule (div ?x (const 2))  (shr ?x (const 1)))
(rule (mod ?x (const 256)) (and ?x (const 255)))

;; LIR level: instruction selection (already working)
(rule (lower (add u8 ?x (const 1)))  (z80_inc ?x))
(rule (lower (load16_le ?base))      (ld_l_hl_inc_ld_h_hl))
```

Same parser (Lanz S-expressions). Same matcher. Same rewriter.
Rules are data, not code — they can be loaded, verified, composed.

### WFC — "Where to assign resources"

Wave Function Collapse propagates constraints bidirectionally.
Each "cell" has a superposition of possible states; constraints narrow them.

```
Dimension 1 (current): instruction position × register assignment
  → forward: def at position i constrains uses at position j>i
  → backward: use at position j constrains def at position i<j
  → collapse: pick minimum-entropy cell, assign, re-propagate

Dimension 2 (proposed): inter-block propagation
  → block arguments carry constraints across edges
  → loop-carried values propagate around back-edges
  → function boundaries propagate calling conventions

Dimension 3 (proposed): cost-weighted collapse
  → instead of "pick first available," pick cheapest
  → cost function considers: T-states, code size, spill pressure
  → Pareto-optimal: multiple objectives, no single best

Dimension 4 (speculative): register class propagation at MIR2
  → MIR2 uses soft class hints (ClassAcc, ClassPointer, ClassGeneral)
  → WFC could propagate these BEFORE PBQP runs
  → reduces PBQP search space dramatically
  → example: if function returns in A, propagate backward:
    every value that flows to the return must be in ClassAcc
```

### PBQP — "At what cost to choose"

Partitioned Boolean Quadratic Programming.
Already used for register allocation (interference graph coloring).
Could also solve:

- **Instruction selection:** each operation has multiple possible patterns,
  each with a cost; connected operations share constraints
- **Calling convention optimization:** each function boundary has a cost
  per register assignment; minimize total call/return overhead
- **Inter-procedural register allocation:** functions that call each other
  should agree on register usage without unnecessary saves

## Multi-Dimensional WFC

The current WFC is 1D: linear instruction sequence within a basic block.
Real programs are graphs (CFGs). The extension:

```
1D:  [inst0] → [inst1] → [inst2] → ...          (current)
2D:  block × instruction                          (inter-block)
3D:  function × block × instruction               (inter-procedural)

Each cell: (block_label, inst_index, operand_slot)
Each cell's superposition: LocSet (bitfield of allowed locations)

Propagation rules:
  forward(block, i, i+1):     same as current
  backward(block, i+1, i):    same as current
  edge(block_a→block_b):      block args carry LocSets across edges
  loop(block_a→block_a):      fixpoint iteration (max N rounds)
  call(caller→callee):        calling convention constraints
  return(callee→caller):      return value location constraints
```

When all cells are collapsed to singletons, the allocation is complete.
When a cell empties (contradiction), the spill recovery fires:
1. Expand the cell to include LocMem (spill slots)
2. Insert load/store moves around the spill point
3. Re-propagate to stabilize

## Gradient Descent for Cost Optimization

When WFC finds multiple valid solutions (no contradictions, multiple
cells have >1 option), gradient descent can pick the cheapest:

```
cost(assignment) = Σ pattern.Cost × frequency
                 + Σ spill.Cost × spill_count
                 + Σ move.Cost × move_count

gradient: perturb one assignment, measure Δcost
step: move toward lower cost
converge: when Δcost < ε or max_iterations reached
```

This is only needed for complex functions where WFC's greedy collapse
produces suboptimal results. For most functions (simple control flow,
few live values), WFC's first solution is already optimal.

## Application at Each Level

### HIR → HIR (source-level optimization)

```lisp
;; Dead code elimination
(rule (if (const true) ?then ?else)  ?then)

;; Loop invariant code motion
;; (would need analysis context, not pure term rewriting)

;; Algebraic simplification
(rule (add (sub ?x ?y) ?y)  ?x)     ;; (x-y)+y → x
(rule (sub (add ?x ?y) ?y)  ?x)     ;; (x+y)-y → x

;; String interpolation optimization
;; (already done, but could be ISLE rules)
```

### MIR2 → MIR2 (IR optimization)

```lisp
;; Constant propagation (replace current Go pass)
(rule (add (const ?a) (const ?b))  (const (+ ?a ?b)))
(rule (cmp.eq ?x ?x)              (const true))
(rule (cmp.ne ?x ?x)              (const false))

;; Branch simplification
(rule (br_if (const true) ?then ?else)   (jmp ?then))
(rule (br_if (const false) ?then ?else)  (jmp ?else))

;; Dead store elimination
;; (needs side-effect analysis — extend ISLE with when-clauses)
```

### LIR → Z80 (instruction selection — already working)

```lisp
(rule (lower (add u8 ?x (const 1)))    (inc ?x))
(rule (lower (mul ?x (const 2)))       (add ?x ?x))
(rule (lower (load16_le ?base))        (ld_l_hl_inc_ld_h_hl))
```

## Decision

**Adopted in principle.** The LIR backend proves the approach works.
Extending to HIR/MIR2 is the next strategic step, replacing hand-written
passes one at a time with ISLE rule sets.

Priority order:
1. ✅ LIR: isel + regalloc (done, 94.6% corpus)
2. MIR2: constant folding + identity simplification (ISLE rules)
3. MIR2: register class propagation (WFC pre-pass before PBQP)
4. HIR: algebraic simplification (ISLE rules)
5. Inter-procedural: calling convention optimization (PBQP)

## Risks

- ISLE rules harder to debug than Go code (mitigated by convergence testing)
- WFC may not converge for pathological CFGs (mitigated by fuel limits)
- PBQP is NP-hard in general (mitigated by small graph size on Z80)
- Premature generalization — should only build what's needed

## References

- Cranelift ISLE: https://cfallin.org/blog/2023/01/20/cranelift-isle/
- PBQP register allocation: Scholz & Eckstein (2002)
- WFC: https://github.com/mxgmn/WaveFunctionCollapse
- E-graphs: https://egraphs-good.github.io/
