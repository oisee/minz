# Phase 4: Related Work Positioning

## Prior Art Map

### 1. Krause 2013 — "Optimal Register Allocation in Polynomial Time" (CC 2013)
**PDF: `/mnt/safe/optimal-register-allocation-in-polynomial-time-1yfn9r69k7.pdf`**

Krause presented a graph-coloring register allocator that optimally handles
spill costs, rematerialization, register preferences, and coalescing for
structured programs (bounded treewidth control-flow graphs) in polynomial time.
The approach uses tree decomposition of the interference graph. It handles
register aliasing (e.g., Z80's B/C as halves of BC). Implementation became
the default allocator in most SDCC backends. Key insight: structured programs
(Algol: tw≤2, C: tw≤7+g where g = goto labels) have low treewidth, enabling
exact DP on tree decompositions.

**Relationship to PFCCO:**
- Krause 2013 solves the *intraprocedural* problem: given a fixed ABI
  (parameter/return register assignment), optimally allocate registers
  within a single function.
- PFCCO solves the *interprocedural* problem: choose the ABI for each
  function to minimize total cost across the program.
- They are **complementary layers**: PFCCO runs first to set ABIs, then
  Krause-style allocation runs per-function with the optimized ABI.
- Nanz's register allocator uses PBQP (not tree decomposition), but the
  insight is the same: the allocator assumes a fixed contract.

**Gap filled by PFCCO:** Krause assumes the calling convention is given.
PFCCO makes the calling convention a first-class optimization variable.

### 2. Krause 2022 — "Efficient Calling Conventions for Irregular Architectures"
**arXiv: 2112.01397**

Empirically evaluated thousands of different C calling conventions for
irregular microcontroller architectures (STM8, Z80, SM83, Rabbit 3000A,
eZ80, TLCS-90) in SDCC. Found substantial improvements over previous
defaults — enough that SDCC changed its default Z80 calling convention
(breaking ABI compatibility). The new Z80 convention: first arg in A (8-bit)
or HL (16-bit), second arg in L (8-bit) or DE (16-bit), return in A/DE/HLDE.

Key methodology: brute-force search over all plausible combinations of
register assignments for parameter positions 1-2 and return value, compiled
against Whetstone, Dhrystone, Coremark, stdcbench benchmarks. One global
convention selected per architecture.

**Relationship to PFCCO:**
- Krause 2022 searches for the best *single* ABI applied to all functions.
  This is a global optimization with N candidates (where N = number of
  distinct ABI configurations tested — "a few thousand" per architecture).
- PFCCO assigns *different* ABIs to different functions. This is strictly
  more expressive: any global ABI is a special case of PFCCO where all
  functions get the same assignment.
- Krause 2022's finding that the best convention "differed a lot from the
  previously used ones" and varied across architectures motivates per-function
  optimization: if no single convention dominates across architectures,
  likely no single convention dominates across functions within a program.
- Krause 2022 Introduction explicitly acknowledges prior work on "choosing a
  calling convention for some individual functions as a link-time optimization"
  (refs [1,3]: De Bus et al. 2004, Caldwell & Chiba 2017 — both for ARM),
  but does not pursue per-function optimization itself, focusing on one-global-
  ABI selection instead.

**Gap filled by PFCCO:** Per-function ABI subsumes global ABI search.
PFCCO provides the per-function optimization that Krause 2022 acknowledges
but leaves to the ARM-focused link-time work (which targets regular RISC
register files, not irregular ones like Z80).

### 2b. De Bus et al. 2004 / Caldwell & Chiba 2017 — Link-Time Per-Function CC
**Referenced in Krause 2022 as [3] and [1]**

De Bus et al. (2004) optimized ARM binary calling conventions at link time.
Caldwell & Chiba (2017) reduced calling convention overhead in OO programs
on ARM Thumb-2. Both target ARM (regular, 16-register RISC file).

**Relationship to PFCCO:**
- Same goal (per-function CC optimization), different setting (RISC vs irregular).
- ARM's regular register file makes the problem primarily about caller/callee-save
  sets, not register *class* assignment.
- PFCCO operates at compile time (before register allocation), not link time.
- Neither provides formal optimality results for DAG call graphs.

**Gap filled by PFCCO:** First formal treatment for irregular architectures
with class-based register semantics (accumulator, counter, pointer).

### 3. Wall 1986 — "Global Register Allocation at Link Time" (SIGPLAN)

Proposed interprocedural register allocation at link time for MIPS (32
general-purpose registers). The algorithm assigns registers to local
variables across procedure boundaries to minimize save/restore overhead.

**Relationship to PFCCO:**
- Wall's work targets RISC architectures with large, regular register files
  (32+ registers). The problem is primarily about minimizing caller/callee-save
  spills across procedure boundaries.
- PFCCO targets architectures with small, irregular register files (Z80: 7
  main registers with semantic constraints). The problem is about matching
  register *classes* (Acc vs Counter vs Pointer) to minimize transfer moves.
- Wall's algorithm works at link time (post-compilation); PFCCO works at
  compile time (pre-register-allocation).

**Gap filled by PFCCO:** Wall doesn't handle irregular register files where
the choice of *which* register (not just *whether* to allocate) changes the
cost. PFCCO's class-based formulation is specifically designed for this.

### 4. MSVC /GL — Per-Function Calling Conventions (Practical)

Microsoft's MSVC with `/GL` (whole-program optimization) can assign custom
calling conventions to internal-linkage functions. Parameters may be passed
in registers instead of on the stack when the compiler determines it's
profitable.

**Relationship to PFCCO:**
- Similar goal: optimize calling conventions for internal functions.
- MSVC targets x86/x64 with many registers and stack-based default ABI.
  The optimization is primarily "stack → register" promotion.
- PFCCO targets Z80 where ALL parameters are already in registers (no stack
  default). The optimization is "which register" not "register vs stack."
- MSVC provides no formal theory or optimality analysis; it's a
  production heuristic.

**Gap filled by PFCCO:** Formal problem statement, complexity analysis, and
optimality proof for the DAG case. First formal treatment of per-function
ABI for register-constrained architectures.

### 5. LLVM IPRA — Interprocedural Register Allocation (2016+)

LLVM includes an IPRA implementation (GSoC 2016, Vivek Pandya). It works by:
1. Collecting per-function clobbered register sets (post-RA analysis)
2. Propagating register usage info bottom-up on the call graph
3. Updating call sites to avoid unnecessary save/restore of registers
   that the callee doesn't actually clobber

**Relationship to PFCCO:**
- LLVM IPRA optimizes **caller/callee-save sets**: if a callee doesn't use
  register R, the caller doesn't need to save R before the call.
- PFCCO optimizes **parameter/return register class assignment**: which
  register each parameter occupies to minimize transfer moves.
- These are orthogonal: IPRA reduces save/restore overhead, PFCCO reduces
  move overhead. Both could be applied together.
- LLVM IPRA targets large RISC register files (x86-64, AArch64) where
  save/restore is the dominant cost. On Z80 (7 registers, no caller-save
  convention), save/restore is rare — the dominant cost is register *class*
  mismatch, which PFCCO addresses.
- LLVM IPRA operates post-RA; PFCCO operates pre-RA (setting the contract
  that the allocator must satisfy).

**Gap filled by PFCCO:** LLVM IPRA doesn't change which registers hold
parameters — it only changes which registers are preserved. PFCCO changes
the parameter assignment itself. On irregular architectures, this is the
larger optimization opportunity.

### 6. US5428793 — Interprocedural Register Allocation Patent

A 1995 patent on interprocedural register allocation that considers procedure
call relationships when assigning registers.

**Relationship to PFCCO:** Historical context. The patent's approach is for
general-purpose RISC machines and focuses on reducing save/restore sequences,
not on register class assignment for irregular files.

### 6. Scholz & Eckstein 2002 — PBQP for Register Allocation

Introduced Partitioned Boolean Quadratic Programming as a formulation for
register allocation. PBQP captures both unary costs (register preferences)
and pairwise costs (interference/affinity) in a single framework.

**Relationship to PFCCO:**
- PFCCO's cost function has natural PBQP structure (unary + edge costs).
- Nanz uses PBQP for intraprocedural allocation already.
- For PFCCO with small candidate spaces, exhaustive enumeration is faster
  than the PBQP solver. But for functions with many parameters (>4), a
  PBQP solver could be used for the interprocedural ABI optimization too.

---

## Positioning Paragraph (for the paper)

> Prior work on register allocation for register-constrained architectures
> has addressed the intraprocedural problem optimally (Krause 2013, CC) and
> the global ABI selection problem empirically (Krause 2022). Link-time
> per-function calling convention optimization has been explored for ARM
> (De Bus et al. 2004, Caldwell & Chiba 2017), while LLVM's IPRA reduces
> caller-save overhead by propagating clobbered-register sets bottom-up on
> the call graph. Wall (1986) and MSVC's /GL handle interprocedural
> register allocation for RISC and x86 architectures with large, regular
> register files. None of these works jointly optimize the calling
> convention *per function* on architectures with small, irregular register
> files where the choice of register class (accumulator vs. counter vs.
> pointer) has semantic implications for instruction selection. We formalize
> this as PFCCO (Per-Function Calling Convention Optimization), prove it
> solvable optimally in O(n) time for acyclic call graphs with bounded
> parameters, and demonstrate its implementation in the Nanz compiler
> targeting Z80, where it produces emergent optimizations including
> zero-byte function aliases.

---

## Gap Analysis Summary

| Work | Scope | ABI Handling | Register File | Optimality |
|------|-------|-------------|---------------|------------|
| Krause 2013 | Intraprocedural | Fixed (input) | Irregular (Z80) | Optimal (proven) |
| Krause 2022 | Global | One ABI searched | Irregular (Z80+) | Best-of-N |
| De Bus 2004 | Link-time per-fn | Stack→reg, save sets | Regular (ARM) | Heuristic |
| Caldwell 2017 | Link-time per-fn | CC overhead reduction | Regular (ARM Thumb-2) | Heuristic |
| LLVM IPRA | Interprocedural | Clobber set propagation | Regular (x86-64+) | Exact (for save/restore) |
| Wall 1986 | Interprocedural | Custom at link time | Regular (MIPS) | Heuristic |
| MSVC /GL | Interprocedural | Stack→reg promotion | Regular (x86) | Heuristic |
| **PFCCO (ours)** | **Interprocedural** | **Per-fn class assignment** | **Irregular (Z80)** | **Optimal for DAGs** |

The unique contribution is the intersection of:
1. **Per-function** granularity (not one global ABI)
2. **Irregular register file** (classes with semantic meaning)
3. **Formal optimality** (proven for acyclic call graphs)
4. **Production implementation** (running in the Nanz compiler)
