# Phase 2: Problem Formalization

## Per-Function Calling Convention Optimization (PFCCO)

### Problem Statement

**Given:**
- A program with call graph G = (F, E), where F is the set of functions
  and E is the set of call edges (possibly weighted by static call count)
- A register file R with physical locations {A, B, C, D, E, H, L, ...}
- For each function f ∈ F:
  - A contract slot vector S(f) = (s₁, ..., sₖ) where each sᵢ is a parameter
    or return value with a type τᵢ ∈ {u8, u16, ptr, bool, flag}
  - A set of plausible register classes per slot:
    - u8:  {ClassAcc(A), ClassCounter(B), ClassGeneral(C/D/E)}
    - u16: {ClassPointer(HL), ClassIndex(DE), ClassPair(BC)}
    - ptr: {ClassPointer(HL), ClassIndex(DE)}
    - flag: {ClassFlag} (fixed, not optimized)
- A cost function:
  - moveCost(c₁, c₂): T-states to transfer a value from a register matching
    class c₁ to one matching class c₂ (on Z80: typically 4T for LD r,r',
    0T when c₁ = c₂)

**Find:**
- For each f ∈ F, an assignment ABI(f): slots → RegClass

**Minimizing:**
```
  Total Cost = Σ_{f∈F} adapterCost(f, ABI(f))
             + Σ_{(caller,callee)∈E} transferCost(ABI(caller), ABI(callee)) × w(e)
```

Where:
- `adapterCost(f, ABI(f))` = cost inside f's body when its parameter is in
  class `c` but the body's first use requires class `c'` (e.g., param in
  ClassGeneral but body does `ADD A,x` requiring ClassAcc)
- `transferCost(ABI(caller), ABI(callee))` = cost of register moves at the
  call site to bridge the caller's assignment to the callee's expected classes
- `w(e)` = weight of edge e (static call count; could be dynamic with PGO)

### Constraint: No Physical Conflicts

Two parameter slots of the same function cannot both be assigned to a class
that uniquely forces the same physical register.

Formally: if `uniquePhys(c)` returns the single forced physical location
for class c (e.g., ClassAcc → A), then for any two params i ≠ j of the same
function, it must NOT be the case that both `uniquePhys(ABI(f).sᵢ)` and
`uniquePhys(ABI(f).sⱼ)` are defined and equal.

Classes with multiple cost-0 locations (e.g., ClassGeneral maps to C, D, or E)
do not uniquely force a register and thus never conflict.

---

## Algorithm: Greedy DP on Topological Order

### Pseudocode

```
PFCCO-Optimize(Module M):
  G ← BuildCallGraph(M)
  order ← TopoSort(G, leaves_first=true)   // Kahn's on reverse graph
  CS ← {}  // ContractSet: f → chosen class vector

  for f in order:
    if f.isExtern or f.isEmpty:
      CS[f] ← currentClasses(f)            // fixed ABI for extern
      continue

    bestChoice ← currentClasses(f)
    bestCost ← Cost(f, bestChoice, G, CS)

    for candidate in CartesianProduct(plausibleClasses(f)):
      if HasParamConflict(candidate):
        continue
      cost ← Cost(f, candidate, G, CS)
      if cost < bestCost:                   // strict < (ties keep current)
        bestCost ← cost
        bestChoice ← candidate

    CS[f] ← bestChoice

  return CS
```

### Cost Function

```
Cost(f, choice, G, CS):
  total ← 0

  // Unary cost: adapter moves inside f
  for each param pᵢ of f:
    natural ← InferNaturalClass(f, pᵢ.reg)
    if choice[i] ≠ natural:
      total += moveCost(choice[i], natural)

  // Unary cost: return adapter moves
  retSites ← count of RET terminators in f
  for each return rⱼ of f:
    natural ← InferNaturalRetClass(f, rⱼ)
    if choice.ret[j] ≠ natural:
      total += moveCost(natural, choice.ret[j]) × retSites

  // Edge cost: call-site transfer moves
  for each CALL instruction in f to callee g:
    if g ∉ CS: continue                    // callee not yet decided
    for each argument aₖ:
      argClass ← choice[k] if aₖ is f's own param, else regInfo(aₖ)
      calleeClass ← CS[g].params[k]
      if argClass ≠ calleeClass:
        total += moveCost(argClass, calleeClass)

  return total
```

### InferNaturalClass (the key bridge function)

```
InferNaturalClass(f, reg r):
  for each instruction in f (block order):
    if r is src[0] of ALU op (ADD/SUB/AND/OR/XOR/CP/MUL):
      return ClassAcc                       // body wants A
    if r is src[0] of Load/Store/Field/PtrBump/PtrAdd:
      return ClassPointer                   // body wants HL
    if r is counter of TermDJNZ:
      return ClassCounter                   // body wants B
    if r is arg[i] of CALL to known callee g:
      return g.Contract.Params[i].Class     // TRANSITIVITY!
  return ClassGeneral
```

The last case (CALL transitivity) is the crucial insight: if a parameter is
simply forwarded to a callee, the natural class is whatever the callee
expects. This creates chains of aligned ABIs through the call graph.

---

## Complexity Analysis

### Per-function cost
O(C × (|body| + |call_sites|))

Where C = |candidates| = Π plausible classes per slot:
- u8 slot: 3 options
- u16 slot: 3 options
- ptr slot: 2 options
- flag slot: 1 (fixed)

For typical Z80 functions (P ≤ 4 params, R ≤ 2 returns):
- C_max = 3⁴ × 3² = 729 before conflict filtering
- After filtering (e.g., no two ClassAcc): typically < 200

### Total complexity
O(|F| × C_max × |body_max|)

For programs with bounded parameter count (P ≤ k), this is **linear in
program size**: O(n) where n = total instruction count.

### Optimality Guarantee

**Theorem:** For acyclic call graphs, PFCCO-Optimize produces an optimal
solution (minimum total cost).

**Proof sketch:**
- Topological order ensures every callee is decided before its callers.
- At each step, the algorithm considers ALL candidate assignments for f.
- The cost function is exact: it accounts for all internal adapter costs
  (unary) and all call-site transfer costs (edge) to already-decided callees.
- Since callees are fixed when we decide f, and f's choice only affects
  f's internal cost and f→callee edges (not callee→callee edges),
  the greedy choice is globally optimal (no future decision can improve
  a past choice, because information flows callee→caller only in a DAG).

This is a standard bottom-up DP argument on a DAG.

**For cyclic graphs (recursion):**
- Cycle members are appended at the end of the topological order.
- Their callees within the cycle are not yet decided when they're processed.
- The algorithm falls back to a heuristic: it uses the current (default)
  contract for the undecided cycle partners.
- Result: optimal for all non-recursive functions, heuristic for
  recursive functions only.

**Possible improvement:** Run Tarjan's SCC decomposition, then within each
SCC, iterate the optimization until convergence (fixpoint). This would give
optimality for mutually-recursive clusters too, at the cost of O(iterations × |SCC|).

---

## Relationship to Known Problems

### Graph Coloring (Register Allocation)
PFCCO is NOT graph coloring. Graph coloring assigns physical registers to
interfering virtual registers within a function. PFCCO assigns register
*classes* to function *interfaces* across functions. They are complementary:
PFCCO runs before intraprocedural register allocation.

### PBQP (Partitioned Boolean Quadratic Programming)
The PFCCO cost function has PBQP structure:
- Unary costs = node costs (adapter moves inside each function)
- Edge costs = pairwise costs (transfer moves between caller/callee)
- Variables = per-function class assignments

The Nanz implementation uses exhaustive enumeration (feasible because the
candidate space is small). For larger candidate spaces, PBQP solvers
(e.g., Scholz & Eckstein 2002) could be used directly.

### ILP (Integer Linear Programming)
PFCCO can be formulated as an ILP:
- Binary variables x_{f,c} = 1 iff function f uses class assignment c
- Σ_c x_{f,c} = 1 for each f (exactly one assignment)
- Minimize Σ unary costs + Σ edge costs

For DAG call graphs, the greedy DP gives the exact optimum, so ILP is
unnecessary. ILP would only help for cyclic call graphs where the greedy
approach is suboptimal.
