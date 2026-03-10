# Register-first parameter passing for MinZ on Z80

**MinZ can and should exploit whole-program visibility to compute mathematically optimal per-function calling conventions — and this is the single highest-leverage optimization available for Z80 code generation.** The Z80's punishingly small register file (7 GP registers, non-orthogonal) means every unnecessary `PUSH`/`POP` pair wastes 21 T-states, every redundant `LD r,r'` wastes 4 T-states, and a conservative fixed calling convention bleeds performance at every call boundary. Because MinZ always rebuilds from source with full program knowledge, it occupies the exact architectural sweet spot for interprocedural register allocation — a class of optimization that SDCC, z88dk, and every other Z80 compiler cannot perform due to separate compilation. This is MinZ's structural advantage, and the current SMC-first approach, while clever, leaves this advantage mostly untapped.

The approach is well-grounded in existing compiler theory: Krause's CC 2013 work proved that provably optimal register allocation is polynomial-time feasible for the Z80's tiny register file, Brisk et al. (2007) showed interprocedural SSA-based allocation yields chordal interference graphs with optimal coloring, and PBQP (Scholz & Eckstein 2002) naturally models the Z80's irregular register constraints via cost matrices. MinZ's MIR pipeline — 118 opcodes, 24 types, 13+ optimizer passes — is the right level to implement this, but the optimization spans two stages and requires careful architectural choices.

---

## Where in the pipeline: the case for a split MIR/codegen strategy

The register-based calling convention optimizer does not fit neatly into a single pipeline stage. It requires information that crystallizes at different points, and the right answer is a **two-phase approach** straddling MIR optimization and MIR→Z80 lowering.

**Phase 1 belongs at MIR-high level** (during or after the existing 13+ optimizer passes). At this stage, the compiler has the complete call graph, knows every function's signature, has performed inlining decisions, and can compute live variable information. This is where to build the interprocedural interference graph, determine which functions are internal vs. external, compute bottom-up register usage summaries, and assign per-function parameter/return-value register slots. The MIR representation is target-aware enough (it's a Z80-focused IR, not an abstract tree) to reason about register classes, but abstract enough that the optimizer operates on logical register assignments rather than physical instruction sequences.

**Phase 2 belongs at MIR-low / codegen level** (during instruction selection and final register allocation). This is where the per-function calling convention assignments from Phase 1 become concrete: the intraprocedural register allocator receives constraints ("parameter 1 arrives in DE, result goes in A, HL is caller-saved") and performs local allocation within those constraints. Peephole optimization at the Z80 assembly level then cleans up any remaining inefficiencies.

**Against putting it all at MIR-high**: The MIR optimizer doesn't currently have a detailed enough cost model for Z80 instruction timing. Assigning "parameter 1 goes in register R3" at MIR level without knowing how R3 maps to physical Z80 register constraints (A is the only accumulator, HL is the only efficient memory pointer) would produce suboptimal results. The Z80's irregularity demands that calling convention assignment be target-aware.

**Against putting it all at codegen**: By codegen time, inlining and fusion decisions have already been made. The calling convention optimizer needs to influence those decisions — for instance, a function that's called once with parameters already in the right registers might not benefit from inlining, while a function requiring expensive parameter shuffling at every call site might benefit enormously. This cross-cutting concern belongs earlier.

**Against a separate "MIR-low" level**: Adding a new IR level adds complexity for uncertain gain. The better approach is to extend MIR with calling convention annotations that the existing codegen phase respects. A function's MIR representation gains metadata: `@convention(param1=HL, param2=A, return=DE, clobbers={A,HL,DE})`. The codegen phase treats these as hard constraints.

The recommended architecture: **interprocedural analysis and convention assignment as a new MIR pass (Phase 1), with convention-aware codegen as an extension to the existing Z80 backend (Phase 2)**. This keeps the pipeline clean and avoids a new IR level.

---

## How to model it: formulating the optimization problem

The whole-program calling convention optimization problem can be formulated as a constrained optimization problem in several mathematically equivalent ways. The core question: for each function f, what assignment of parameters and return values to physical Z80 registers minimizes total program cost (T-states or bytes)?

### The PBQP formulation (recommended)

PBQP (Partitioned Boolean Quadratic Programming) is the natural fit for the Z80's irregular register file. Construct a PBQP instance as follows:

**Nodes**: One node per function parameter and return value across the entire program. For a program with N functions where function fᵢ has kᵢ parameters and rᵢ return values, there are Σ(kᵢ + rᵢ) nodes.

**Domain per node**: The set of physical Z80 locations where this value could reside: {A, B, C, D, E, H, L, IXH, IXL, IYH, IYL, stack+0, stack+2, ...} for 8-bit values; {BC, DE, HL, IX, IY, stack+0, ...} for 16-bit values.

**Unary cost vectors**: For each node, a vector giving the cost of assigning each physical location. These encode: (a) how the callee uses the value — if the first thing f does is `ADD A, param1`, then assigning param1 to A has cost 0 while assigning it to B costs 4T (for LD A,B); (b) register class constraints — 16-bit pointer parameters that need (HL) addressing strongly prefer HL.

**Pairwise cost matrices**: For each pair of connected nodes (parameters of the same function, or a return value and the corresponding use at a call site), a matrix encoding: (a) interference — two live-at-the-same-time values cannot share a register (∞ cost); (b) copy costs — if caller has value V in register R1 and callee expects it in R2, the shuffle cost appears here (4T for LD r,r'; 4T for EX DE,HL; 11/10T for PUSH/POP pairs); (c) register aliasing — B and C conflict with BC as a 16-bit pair.

**Objective**: Minimize the total cost, which is the sum of all unary costs plus all pairwise costs, weighted by estimated execution frequency of each call edge.

The PBQP solver runs in nearly linear time for most practical instances (reduction rules R0, R1, R2 handle most nodes; the RN heuristic handles the rest). For the Z80 with ~12 candidate locations per node and typical programs with a few dozen functions, **the PBQP instance is tiny — solvable optimally in milliseconds**.

### The ILP formulation (for provably optimal solutions)

Following Kong & Wilken (1998), formulate as a 0-1 integer linear program:

**Decision variables**: xᵥ,ᵣ ∈ {0,1} = 1 iff value v is assigned to register r.

**Constraints**:
- Assignment: Σᵣ xᵥ,ᵣ = 1 for all v (every value gets exactly one location)
- Interference: xᵤ,ᵣ + xᵥ,ᵣ ≤ 1 for all interfering (u,v) and register r
- Register class: xᵥ,ᵣ = 0 if r ∉ class(v)
- Aliasing: xᵤ,B + xᵥ,BC ≤ 1 (B conflicts with BC pair)

**Objective**: Minimize Σ_edges (shuffle_cost × frequency), where shuffle_cost is a linear function of the x variables.

Kong & Wilken showed ILP is actually **faster for irregular architectures with few registers** — the constraint matrix is small. For Z80 programs with ~50 functions and ~7 registers, this is a few hundred variables and constraints, solvable by any LP solver (even a simple branch-and-bound) in under a second.

### The graph-theoretic formulation

Build an interprocedural interference graph where nodes are all parameter/return-value live ranges across the program. Color this graph with Z80 physical registers. The SSA form guarantee (Brisk et al. 2007) gives chordality, enabling optimal polynomial-time coloring via Maximum Cardinality Search + greedy assignment.

However, the Z80's pre-coloring constraints (A for ALU, HL for addressing) break pure chordality. The practical approach: use SSA-based spilling first (reduce register pressure to ≤ k everywhere), then use PBQP for assignment.

### The dynamic programming formulation

For call chains (not call graphs with complex SCCs), a DP over the call tree works: let `cost(f, convention_f)` be the minimum cost of executing function f and all its callees, given that f uses calling convention `convention_f`. The recurrence:

```
cost(f, conv_f) = local_cost(f, conv_f) + Σ_{g ∈ callees(f)} min_{conv_g} [shuffle_cost(conv_f → conv_g) × freq(f→g) + cost(g, conv_g)]
```

This is solvable bottom-up on the call tree in O(N × C²) where N is the number of functions and C is the number of candidate conventions per function. For Z80 with ~100 reasonable conventions per signature and ~50 functions, this is trivially fast. **For strongly connected components (recursive functions), iterate until convergence or use a fixed convention within the SCC.**

---

## Z80-specific constraints that shape the solution space

The Z80's register architecture creates a uniquely constrained optimization landscape. Every constraint below must be encoded in whichever formulation is chosen.

### Register pair atomicity

B/C, D/E, and H/L form 16-bit pairs BC, DE, HL. This creates aliasing constraints: if a function receives an 8-bit parameter in B and a 16-bit parameter in DE, it cannot also receive a third 8-bit parameter in D or E. The aliasing matrix is small (6 aliasing pairs for main registers) but must be tracked precisely. **PUSH/POP operate only on pairs** (11/10T), so saving a single 8-bit register costs the same as saving two — this means packing two 8-bit parameters into one pair (e.g., B and C for two u8 args) amortizes save/restore cost.

### The accumulator monopoly

**A is the only destination for ADD, SUB, AND, OR, XOR, CP, and IN.** Any function that performs arithmetic will need values in A. The optimal convention should route the "primary operand" — the value most frequently used in ALU operations — directly into A. Krause's empirical analysis confirmed this: **returning 8-bit values in A (not L as the old SDCC convention did) improved performance across all benchmarks**. Similarly, if a function's first action is `CP param`, that parameter should arrive in A.

### HL as the precious pointer register

HL is the only register pair providing efficient indirect memory access at **7T per LD r,(HL)** versus 19T for LD r,(IX+d). Functions that dereference pointers should receive those pointers in HL. Functions that perform 16-bit addition should receive the accumulator operand in HL (ADD HL,rr exists; ADD BC,rr does not). The calling convention optimizer should perform a lightweight use-analysis of each function body to determine whether a parameter is used primarily as a pointer (→ assign to HL) or as data (→ assign to DE or BC).

### IXH/IXL/IYH/IYL: four bonus registers at a price

The undocumented index register halves provide **4 additional 8-bit registers** at a +4T penalty per instruction (8T vs 4T for main registers). They are reliably available on all Z80 silicon. However, Krause's SDCC analysis deliberately excluded IX/IY from calling conventions because some platforms (ZX Spectrum, CP/M) reserve IY for the OS. MinZ should take a more aggressive stance: **if the target platform doesn't reserve IX/IY (e.g., bare-metal, Agon Light), include IXH/IXL/IYH/IYL in the allocation pool.** Make this configurable per target. When available, these registers are ideal for "cool" values — parameters that are read once or twice but not in tight loops. The +4T penalty is far cheaper than a stack spill (PUSH+POP = 21T minimum, plus addressing overhead).

### The alternate register set: high reward, hidden cost

EXX (4T) swaps BC/DE/HL with their shadows, providing 6 additional 8-bit registers. EX AF,AF' (4T) adds A'. On paper, this doubles the register file. In practice, several constraints limit usefulness:

**You cannot address shadow registers individually.** To move a value from L to L', you need `LD A,L / EXX / LD L,A / EXX` (16T) or `PUSH HL / EXX / POP HL / EXX` (29T). This means shadow registers are best used as a **block context switch**, not for individual parameter passing.

**Interrupt handlers often use shadow registers.** If the system uses IM2 interrupts with EXX-based register saving (common on ZX Spectrum), shadow registers are off-limits unless interrupts are disabled. The convention optimizer needs a global flag: `shadows_available: bool`.

**The optimal use case for shadows**: Functions that perform two-phase computation (e.g., process input, then produce output) where the input and output register sets don't overlap. Swap to shadows at the phase boundary. Alternatively, use shadows to hold **callee-saved values across inner function calls** — swap to shadows before CALL, swap back after — at only 8T total (EXX before + EXX after) versus 42T+ for pushing and popping 3 register pairs.

**Recommendation for MinZ**: Model shadow registers as a **separate register bank** in the PBQP formulation. Accessing bank 1 (shadows) from bank 0 (main) incurs a 4T EXX cost. The solver will naturally find cases where the bank-switch cost is amortized over multiple operations. Do not try to use individual shadow registers as general-purpose storage — the transfer cost makes this uneconomical except for the specific pattern of using A as a bridge at 12T per byte.

### Quantitative cost model for the optimizer

Every decision in the register convention optimizer should be driven by a cost model. The key T-state costs for parameter-passing operations:

| Operation | T-states | Bytes | Use case |
|-----------|----------|-------|----------|
| LD r,r' (register shuffle) | 4 | 1 | Moving param between regs |
| EX DE,HL (swap) | 4 | 1 | Swapping pointer with data |
| EXX (bank switch) | 4 | 1 | Shadow register access |
| PUSH rr + POP rr (spill/restore) | 21 | 2 | Saving across calls |
| PUSH IX + POP IX | 29 | 4 | Index register spill |
| LD r,IXH (index half access) | 8 | 2 | Using index halves |
| CALL nn + RET (call overhead) | 27 | 4 | Function call baseline |
| SMC patch: LD (nn),A + LD A,n | 20 | 6 | SMC parameter write |

---

## External function boundaries: contracts vs. adapters

When the whole-program optimizer encounters a function it cannot analyze — ROM calls, interrupt vectors, hand-written assembly, or future library functions — it faces a boundary problem. Two strategies exist, and the right choice depends on call frequency.

### Inline contract resolution

For each external function, the compiler knows (or the programmer declares) a fixed calling convention: "ROM print expects character in A, preserves BC/DE." The optimizer treats this as a **hard constraint** at every call site: the interprocedural convention assignment must ensure that at the call site, the value is already in A (or insert a minimal shuffle). This is zero-cost at the call boundary — all cost is absorbed by the surrounding register allocation.

**When this makes sense**: Hot call sites (called in loops), external functions with simple conventions (1-2 register parameters), ROM/BIOS calls with fixed APIs. This is the common case for Z80 programs. MinZ already supports `@extern(0x0010) fun rom_print()` — the convention optimizer should parse these annotations and treat them as pre-colored nodes in the interference graph.

### Adapter shims (thunks)

For external functions with conventions that conflict badly with the optimal internal convention, generate a small wrapper that shuffles registers. Example: if internal convention has a pointer in DE but the external function expects it in HL, generate:

```z80
_adapter_ext_func:
    EX DE,HL        ; 4T, 1 byte
    CALL ext_func   ; 17T, 3 bytes  
    EX DE,HL        ; 4T, 1 byte
    RET             ; 10T, 1 byte
```

Total overhead: **8T and 2 bytes** beyond the normal CALL/RET. For an EX DE,HL swap, this is trivial. For more complex shuffles, the adapter grows but remains small.

**When this makes sense**: Cold call sites (called once or rarely), external functions with complex conventions (many parameters needing rearrangement), or when the adapter enables a significantly better internal convention that saves more than it costs across all other call sites.

**The optimizer should compute both options** for every external call edge and pick the cheaper one. This is a simple binary decision per call site: `cost(inline_shuffle × frequency) vs. cost(adapter_overhead × frequency + adapter_code_size)`. For hot calls, inline resolution wins. For cold calls, adapters keep the hot code clean.

### A third option: convention polymorphism

For external functions called from multiple internal sites with different register contexts, generate **multiple adapters** — one per distinct calling context. Each adapter shuffles from the caller's context to the external convention. The optimizer selects the adapter at each call site. This increases code size but can be optimal for programs with diverse call patterns to the same external function.

---

## SMC-first vs. register-first: a critical reassessment

The TSMC (True Self-Modifying Code) calling convention is MinZ's signature innovation. It patches parameter values directly into callee instruction immediates, turning `LD A, 0` into `LD A, 42` at runtime. The claimed benefit: 7-20T per parameter versus 44T+ for memory reads. This deserves rigorous scrutiny given the alternative of register-based passing with whole-program optimization.

### Where SMC genuinely wins

**Constant-like parameters in tight loops**: When a parameter doesn't change between iterations (e.g., a threshold in a filter lambda), SMC eliminates the parameter load entirely — the value is baked into the instruction. The equivalent register-based approach would hold the value in a register, which is also free to read but **consumes a register for the entire loop's lifetime**. On the Z80's 7-register file, holding one constant in a register might force a spill elsewhere that costs more than the SMC patch.

**Iterator fusion with inline lambdas**: MinZ's fusion optimizer inlines lambda bodies into DJNZ loops. With SMC, `filter(|x| x > N)` compiles to `CP N+1 / JR C` where N is patched once before the loop. The register-first alternative would load N into a register before the loop — equally fast during iteration, but requiring one more register. This is a legitimate advantage when register pressure is already extreme.

### Where SMC loses to register-based passing

**The patch cost itself**: Writing a parameter via SMC requires a memory store to the instruction stream: `LD (callee_param_addr), A` costs 13T and 3 bytes. For multiple parameters, each costs 13T. A register-based convention with parameters arriving in the right registers costs **0T** for parameter passing — the values are simply already there. Even with shuffling, `LD B,A` is 4T versus 13T for an SMC patch.

**Cache and prefetch effects on modern Z80 variants**: The eZ80, Z80N (Spectrum Next), and Rabbit processors have instruction prefetch buffers. Self-modifying code invalidates these buffers, potentially stalling the pipeline. On original Z80 silicon there's no prefetch, so this doesn't apply — but it limits MinZ's portability story.

**Code cannot live in ROM**: SMC functions must be in RAM. This matters for CP/M systems with banked memory, ROM-based systems, and any platform where code is loaded into write-protected memory.

**Re-entrancy is impossible**: An SMC function cannot be called recursively or from an interrupt handler while in use. The parameters are patched into a single copy of the code. Register-based conventions are inherently re-entrant.

**Binary size**: Each SMC patch site requires a 3-byte `LD (nn), r` instruction in the caller. For a function with 3 parameters called from 5 sites, that's 45 bytes of patching code. Register-based passing with well-chosen conventions might need zero shuffle bytes at many call sites.

### The synthesis: SMC as a targeted optimization, not a default

With whole-program register allocation, **register-first should be the default calling convention**, and SMC should be an escape hatch for specific patterns where it provably wins. The optimizer should:

1. First compute optimal register-based conventions for all internal functions
2. Identify functions where a parameter is (a) infrequently changed between calls and (b) used as an immediate in the callee's hot loop — these are SMC candidates
3. Compare the total cost: SMC patch overhead at all call sites versus the register pressure cost of holding the value in a register throughout the callee
4. Apply SMC selectively, only where the cost model confirms a net win

This hybrid approach preserves MinZ's innovative SMC capability while capturing the larger wins from interprocedural register optimization. **For most functions, register-based passing with whole-program-optimal conventions will outperform SMC.** SMC's advantage is narrow: constant-like parameters in tight inner loops where register pressure is already maximal.

The current "SMC-first" philosophy is understandable given MinZ's history — SMC is an impressive demo and a genuine Z80 optimization — but with whole-program visibility, it's like optimizing the curtains when you could be redesigning the foundation.

---

## Existing theory that maps directly to this problem

The compiler literature provides a rich toolkit for MinZ's problem. Here is what maps well and what doesn't.

### Krause's polynomial-time optimal allocation (CC 2013)

Philipp Klaus Krause — the SDCC maintainer — proved that **provably optimal register allocation is achievable in polynomial time** for structured programs on small irregular register files. His allocator, already shipping in SDCC, handles register aliasing, pre-coloring, and the Z80's non-orthogonal instruction set. The runtime is O(n · pw^r · r^(pw·r+r+1)) where n is code size, r ≈ 7 registers, and pw is the pathwidth of the control flow graph (typically 2-4 for real functions). This means **SDCC already performs optimal intraprocedural allocation for the Z80**. MinZ should not try to beat this for individual functions — instead, it should focus on the interprocedural dimension that SDCC cannot exploit due to separate compilation.

### Krause's calling convention search (arXiv 2112.01397)

Krause empirically evaluated **thousands of fixed calling conventions** for Z80 across four benchmarks (Whetstone, Dhrystone, Coremark, stdcbench). The optimal fixed convention (__sdcccall(1)) passes the first 8-bit parameter in A, first 16-bit in HL, second in DE, and returns 8-bit in A, 16-bit in DE. This yielded **3-15% improvement** in both code size and speed — significant enough that SDCC broke ABI compatibility to adopt it.

Crucially, Krause deliberately excluded IX/IY and shadow registers from the search, and searched only for a single fixed convention. MinZ can go further in both dimensions: include IX halves in the allocation pool (where platform-safe), and use per-function rather than fixed conventions. **Krause's result is the floor; MinZ should treat it as the starting point and optimize beyond it.**

### PBQP for irregular architectures (Scholz & Eckstein 2002)

PBQP was invented for exactly this class of problem: register allocation on processors where registers are non-interchangeable and instruction costs vary by register. The Z80 is a textbook irregular architecture. PBQP's cost-matrix formulation can encode every Z80 quirk: A-only ALU operations, HL-only addressing, pair aliasing, IX/IY prefix penalties. The solver runs in near-linear time and finds near-optimal solutions (97.4% optimal on SPEC2000). **PBQP should be the core algorithm for MinZ's intraprocedural register assignment phase.**

### Interprocedural register allocation (Wall 1986, Brisk 2007)

Wall's 1986 link-time register allocation and Brisk's 2007 polynomial-time interprocedural SSA allocation provide the theoretical foundation for whole-program calling convention optimization. The key insight from both: **process the call graph bottom-up, computing per-function register usage summaries, then assign calling conventions top-down based on actual register needs.** LLVM's IPRA pass implements a lightweight version of this (propagating actual clobber masks, not reassigning conventions) — MinZ should implement the full version.

### What doesn't map well

**Classic Chaitin-Briggs graph coloring** assumes interchangeable registers and struggles with the Z80's irregularity. The spilling heuristics degrade badly when k is very small (4-7 colors). This is the wrong approach for Z80 — Krause's pathwidth-based optimal allocator and PBQP both dominate it.

**Linear scan allocation** is fast but too imprecise for a register file this small. With only 7 registers, the quality difference between optimal and linear-scan allocation is proportionally much larger than on a 32-register RISC. Linear scan makes sense for JIT compilers where compilation speed matters; MinZ rebuilds from source anyway, so compilation speed is not the binding constraint.

---

## Specific recommendations for MinZ's MIR architecture

Based on everything above, here is a concrete roadmap for implementing register-first parameter passing in MinZ.

### Step 1: Add calling convention metadata to MIR functions

Extend the MIR function representation with a `CallingConvention` struct:

```go
type CallingConvention struct {
    Params    []PhysLocation  // where each parameter arrives
    Returns   []PhysLocation  // where each return value goes  
    Clobbers  RegisterMask    // which registers this function destroys
    Preserved RegisterMask    // which registers are guaranteed preserved
    UseSMC    []bool          // per-parameter: use SMC instead of register?
}
```

Initially, all functions get the default TSMC convention. The optimizer progressively refines these.

### Step 2: Build the call graph with edge weights

After inlining and fusion decisions are finalized, construct the complete call graph. Weight each edge by estimated execution frequency (loop nesting depth is a good proxy: weight = 10^depth). Mark external function edges with their fixed conventions.

### Step 3: Bottom-up register usage analysis

Process functions in reverse topological order (leaves first). For each function, perform a quick register usage analysis: which physical registers does it read, write, and clobber? For callees, incorporate their clobber information transitively. This gives an exact `RegisterMask` per function — the same information LLVM's IPRA computes, but at MIR level before final codegen.

### Step 4: Formulate and solve the PBQP instance

Construct the interprocedural PBQP problem as described in the "How to model it" section. For programs with ≤100 functions (nearly all Z80 programs), the solver runs in milliseconds. Assign the resulting conventions to each function's MIR metadata.

### Step 5: Convention-aware codegen

Extend the Z80 codegen to respect `CallingConvention` metadata. At function entry, parameters are already in the specified registers — no prologue shuffle needed. At call sites, the codegen inserts minimal shuffle instructions to place arguments into the callee's expected registers. The intraprocedural register allocator (ideally PBQP or Krause-style optimal) works within the constraints imposed by the interprocedural convention.

### Step 6: SMC as a post-pass optimization

After register-based conventions are assigned, identify SMC candidates: parameters that are (a) compile-time constant or loop-invariant at call sites and (b) used as immediates in the callee's hot path. For these specific parameters, switch from register-based to SMC passing. Recompute the affected functions' conventions since this frees a register.

### Step 7: Iterate if necessary

Convention assignment and intraprocedural allocation are interdependent. A change in calling convention affects register pressure, which might change clobber sets, which might change conventions. In practice, **one or two iterations suffice** — the Z80's small register file limits the space of possible conventions, so convergence is fast.

### What to defer

Shadow register optimization (EXX-based bank switching) is valuable but complex. It requires tracking which bank is active at every program point and introduces subtle bugs with interrupt handlers. Recommend deferring this to a later phase and focusing first on main registers + IX halves. The infrastructure for shadow registers can be designed now (the `PhysLocation` type should include bank information) but implementation can wait.

### A note on the stale HL tracking bug (ADR-0006)

The known register allocator bug — stale HL tracking in loops causing wrong results — is directly relevant. Any register convention optimizer that assigns HL as a parameter register must correctly track HL's liveness across loop back-edges. This bug should be fixed first; it will bite harder with interprocedural conventions because HL will be assigned more aggressively. The fix likely involves invalidating HL tracking at loop headers (inserting phi-like nodes for HL in the SSA representation). Fixing this bug is a prerequisite, not a follow-on.

---

## Conclusion: MinZ's unfair advantage

MinZ occupies a unique position in the Z80 compiler landscape. It always has whole-program visibility, its programs are small enough for exact optimization, and the Z80's tiny register file means every optimization matters proportionally far more than on modern architectures. **The PBQP-based interprocedural register convention optimizer described here should yield improvements exceeding Krause's 3-15% fixed-convention gains** — because it eliminates not just the overhead of a bad fixed convention, but the concept of a fixed convention entirely.

The recommended priority order: fix ADR-0006 → add CallingConvention metadata to MIR → implement bottom-up register usage analysis → implement the PBQP solver → make codegen convention-aware → selectively apply SMC as a post-pass. Each step delivers value independently and can be validated with the existing MIR→Z80→binary→emulate test pipeline.

The SMC innovation remains valuable — it's the right tool for constant-like parameters in tight loops — but it should be a scalpel, not the default. **Register-first, SMC-where-proven is the correct engineering stance** for a whole-program compiler with the analytical tools to know the difference.