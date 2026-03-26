# Paper E: Bounded-Type GPU Iterator Fusion — Implicit Parallelism from Type Constraints

**Authors:** Alice Vinogradova, with Claude Opus 4.6 (AI collaborator)

**Status:** Research idea / outline

---

## Core Insight

Languages with bounded-range types (u8 = 0..255, u16 = 0..65535) have an implicit GPU parallelism opportunity: iterator chains over bounded ranges can be compiled to GPU kernels where each thread processes one element. No programmer annotation needed — the compiler infers parallelism from the type system.

```nanz
// Source: looks sequential
(0..255).iter()
    .map(|x| x * 3 + 1)
    .filter(|x| x < 100)
    .count()

// Compiled to: 256 GPU threads
// Thread i: compute f(i), test predicate, atomic_add if passes
// Result: exact count, computed in parallel
```

## The Connection to Superoptimization

This is exactly what our z80-optimizer already does:

```
// Superoptimizer: verify mul×3 for all 256 inputs
(0..255).map(|x| simulate(mul3_sequence, x))
        .all(|result| result == x * 3)
// → 256 GPU threads, each verifies one input
// → atomic: all pass? → sequence is correct
```

The superoptimizer's exhaustive search IS bounded-type iterator fusion. We just didn't frame it that way.

## Three Levels

### Level 1: gpugen DSL (superoptimizer-specific)

ISA definition as Go struct → emit CUDA/Metal/OpenCL kernels.

```go
var Z80Mul = ISA{
    Name: "z80_mulopt",
    State: []Reg{{Name: "a", Type: U8}, {Name: "b", Type: U8}},
    Ops: []Op{
        {Name: "ADD A,B", Cost: 4, Body: `r=(u16)a+b; carry=r>0xFF; a=(u8)r;`},
        {Name: "LD B,A",  Cost: 4, Body: `b=a;`},
    },
}
gpugen.Emit(Z80Mul, gpugen.CUDA)   // → z80_mulopt.cu
gpugen.Emit(Z80Mul, gpugen.Metal)  // → z80_mulopt.metal
```

**Status:** Designed, ready to implement. z80-optimizer session building it.

### Level 2: Bounded-range iterator fusion (compiler-automatic)

The compiler detects when an iterator chain meets three conditions:
1. **Source is bounded range** — u8 (256 elements), u16 (65K)
2. **Body is pure** — no side effects, no globals, no I/O
3. **Collect is reducible** — count, sum, min, max, filter+collect

When all three hold, compile the chain to a GPU kernel automatically.

```
Nanz source              Compiler analysis              Target
─────────────            ──────────────────             ──────
(0..255).map(f)          range≤256, f pure              → GPU (256 threads)
(0..65535).map(f)        range≤65K, f pure              → GPU (65K threads)
arr.iter().map(f)        arr.len unknown, f pure        → GPU if len > threshold
(0..N).map(f)            N runtime, f has effects       → CPU sequential
```

**Pipeline parallelism** for multi-stage chains:

```
Stage 1 (GPU kernel):  source.map(|x| f(x))     → buffer A (device memory)
Stage 2 (GPU kernel):  bufferA.filter(|x| g(x))  → buffer B (compacted)
Stage 3 (GPU kernel):  bufferB.reduce(|a,b| a+b) → scalar result
```

Each stage is a separate kernel dispatch. Inter-stage data stays on GPU (no host round-trip).

**CSP primitives** for coordination:
- Channels between stages = device memory buffers
- Barriers = kernel synchronization points
- Fan-out/fan-in = thread group topology

### Level 3: mir2cuda — general Nanz→GPU backend

Full compiler backend: MIR2 → PTX (CUDA) / SPIR-V (Vulkan/OpenCL) / Metal IR.

```bash
mz program.nanz -b cuda -o program.ptx    # CUDA
mz program.nanz -b vulkan -o program.spv  # Vulkan/SPIR-V
mz program.nanz -b metal -o program.ir    # Metal
```

This is a major project (weeks). The MIR2 IR already has the right ops (add/sub/mul/cmp/load/store/call). The z80codegen is ~1500 LOC — a cuda codegen would be similar. But GPU programming has fundamentally different concerns (thread divergence, shared memory, coalesced access) that MIR2 doesn't model.

## Why Bounded Types Matter

Traditional GPU programming requires explicit parallelism (CUDA threads, OpenCL work items). The programmer must decide what runs in parallel.

Bounded types invert this: the TYPE tells the compiler the parallelism. `u8` means "at most 256 values." An iterator over u8 is inherently parallelizable to 256 threads.

```
Type        Range       GPU threads     Time complexity
u8          0..255      256             O(1) per element
u16         0..65535    65,536          O(1) per element
u8 × u8    0..65535    65,536          O(1) per pair
u8 × u8 × u8  0..16M  16,777,216      O(1) per triple
```

For exhaustive verification (property testing, superoptimization), bounded types give COMPLETENESS: "for ALL possible inputs" is a finite, GPU-parallelizable computation.

## The Meta-Circular Connection

The MinZ compiler uses GPU tables (83.6M entries) for register allocation. These tables were computed by exhaustive GPU search over bounded constraint spaces. If the compiler itself could compile its own table-building code to GPU kernels, we'd have:

```
MinZ compiler (CPU)
  → compiles VIR table builder (Nanz) to GPU kernel
  → GPU kernel exhaustively solves all register allocation shapes
  → results shipped back as table
  → MinZ compiler uses table for O(1) allocation

The compiler builds its own brain on GPU.
```

This is a Futamura projection applied to GPU compilation: partially evaluate the compiler with respect to the ISA, compile the residual to GPU, ship the results back.

## Comparison with Existing Approaches

| Approach | Parallelism | Bounded types? | Exhaustive? |
|----------|-------------|----------------|-------------|
| CUDA/OpenCL | Explicit (programmer) | No | Manual |
| Futhark | Implicit (map/reduce) | No | No |
| Halide | Scheduling DSL | No | No |
| Julia GPU | @cuda annotation | No | No |
| **MinZ (proposed)** | **Implicit (type-derived)** | **Yes** | **Yes (for bounded)** |

The key differentiator: bounded types guarantee FINITE search spaces. GPU parallelism + finite space = exhaustive verification. No other approach offers this.

## Implementation Sketch

### Phase 1: Compiler analysis

```go
// In iterator fusion pass (pkg/vir/isle.go or pkg/hir/iter.go)
func canGPUFuse(chain IteratorChain) bool {
    return chain.Source.IsBoundedRange() &&
           chain.Body.IsPure() &&
           chain.Collect.IsReducible()
}
```

### Phase 2: Kernel generation

```go
// Generate GPU kernel from iterator chain
func emitGPUKernel(chain IteratorChain, backend GPUBackend) string {
    // Thread ID = element index
    // Body = per-thread computation
    // Collect = reduction (atomic or tree-based)
    return backend.EmitKernel(KernelDesc{
        ThreadCount: chain.Source.Range().Size(),
        Body:        chain.Body.ToCExpr(),
        Reducer:     chain.Collect.Reducer(),
    })
}
```

### Phase 3: Host integration

```go
// In compiler pipeline: detect GPU-fusible chains, emit kernel + host dispatch
func compileIteratorChain(chain IteratorChain) {
    if canGPUFuse(chain) && gpuAvailable() {
        kernel := emitGPUKernel(chain, detectGPUBackend())
        emitHostDispatch(kernel, chain.Source.Range())
    } else {
        // Fall back to CPU DJNZ loop (existing MinZ iterator codegen)
        emitCPULoop(chain)
    }
}
```

## Open Questions

1. **Threshold**: At what range size does GPU dispatch beat CPU? For Z80 target: always CPU (no GPU on Z80!). For host-side compilation: GPU for ranges > ~1000 elements.

2. **Purity analysis**: How deep does the purity checker go? Calls to pure functions? Recursive functions? The analysis must be conservative — any doubt → CPU.

3. **Multi-dimensional**: `(0..255).flat_map(|x| (0..255).map(|y| f(x,y)))` — 65K threads. This is the superoptimizer's interference enumeration.

4. **Reduction strategies**: Count/sum → atomic. Min/max → atomic. Filter+collect → parallel compact (prefix sum). Sort → bitonic sort on GPU.

5. **Memory model**: Bounded types mean bounded memory. A u8→u8 lookup table is 256 bytes — fits in shared memory. u16→u8 is 64KB — fits in L1 cache on GPU.

---

*From bounded types to implicit GPU parallelism: the compiler infers what the programmer means.*
