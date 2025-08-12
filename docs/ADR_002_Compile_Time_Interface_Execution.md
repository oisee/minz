# ADR-002: Compile-Time Interface Execution

**Status**: Proposed  
**Date**: 2025-08-11  
**Decision**: Implement compile-time execution for interfaces in MinZ v0.12.0

## Context

After successfully implementing compile-time interface casting in v0.11.0 (zero runtime overhead), we've identified the next evolutionary step: **compile-time execution** of interface methods. This would allow interfaces to not just dispatch at compile-time, but actually EXECUTE at compile-time when conditions permit.

Currently:
- Cast interfaces resolve dispatch at compile-time ✅
- Method calls compile to direct CALLs ✅  
- But the actual code still runs on Z80 at runtime

The opportunity:
- Execute interface methods during compilation when inputs are const
- Generate specialized implementations based on usage patterns
- Eliminate entire code paths that are never taken
- Prove correctness at compile-time

## Decision

Implement a comprehensive compile-time execution system for interfaces with the following capabilities:

### Core Features

1. **@execute** - Constant evaluation of interface methods
2. **@specialize** - Type-specific optimizations
3. **@proof** - Compile-time contract verification
4. **@derive** - Automatic implementation generation
5. **@analyze_usage** - Usage pattern optimization
6. **@compile_time_vtable** - Complete dispatch elimination

### Implementation Strategy

Phase 1 (v0.12.0): Foundation
- @execute for pure functions with const inputs
- Basic @specialize for common types
- Infrastructure for compile-time evaluation

Phase 2 (v0.13.0): Advanced Features  
- @proof system for contract verification
- @derive for automatic implementations
- Usage analysis framework

Phase 3 (v0.14.0): Full Power
- @compile_time_vtable with complete elimination
- Self-modifying interface optimization
- Meta-metaprogramming capabilities

## Consequences

### Positive

✅ **Negative-Cost Abstractions**: Code that doesn't even exist at runtime
✅ **Smaller Binaries**: Dead code elimination at interface boundaries
✅ **Faster Execution**: Const propagation through interfaces
✅ **Compile-Time Safety**: Provable interface contracts
✅ **Developer Productivity**: Auto-generation of boilerplate

### Negative

❌ **Compilation Time**: More work at compile-time
❌ **Complexity**: Sophisticated compiler infrastructure needed
❌ **Debugging**: Harder to debug code that doesn't exist
❌ **Learning Curve**: New concepts for developers

### Neutral

➖ **Binary Size vs Speed Tradeoff**: Specialization can increase size
➖ **Predictability**: Different builds might have different performance
➖ **Tooling Requirements**: Need better IDE support

## Technical Approach

### 1. Compile-Time Interpreter

Build a MIR interpreter that can execute pure functions:
```go
type CompileTimeExecutor struct {
    mir         *MIR
    constPool   map[string]Value
    pureCache   map[string]bool
}
```

### 2. Purity Analysis

Determine which functions can be executed at compile-time:
```go
func isPure(fn *Function) bool {
    // No side effects, no I/O, no global state
    // Only depends on parameters
}
```

### 3. Const Propagation

Track const values through interface boundaries:
```go
type ConstTracker struct {
    values     map[Variable]ConstValue
    interfaces map[Interface]ConstState
}
```

### 4. Specialization Engine

Generate optimized versions for specific types:
```go
type Specializer struct {
    patterns   []UsagePattern
    threshold  int  // Minimum usage to specialize
}
```

### 5. Proof Checker

Verify interface contracts at compile-time:
```go
type ProofChecker struct {
    invariants []Invariant
    solver     SMTSolver
}
```

## Examples

### Basic @execute
```minz
interface Calculator {
    @execute when const
    fun calculate(x: u8) -> u8;
}

// This disappears at compile-time!
const result = calculator.calculate(42);  // Becomes: const result = 84
```

### Type Specialization
```minz
interface Drawable {
    @specialize for ["Circle", "Rectangle"] {
        Circle -> @unroll
        Rectangle -> @vectorize
    }
    fun draw() -> void;
}
```

### Contract Verification
```minz
interface Sortable {
    @proof {
        antisymmetric: compare(a,b) == -compare(b,a)
        transitive: compare(a,b) < 0 && compare(b,c) < 0 => compare(a,c) < 0
    }
    fun compare(other: Self) -> i8;
}
```

## Metrics for Success

| Metric | Target | Measurement |
|--------|--------|-------------|
| Binary size reduction | 15-30% | Compare with/without CTE |
| Performance improvement | 20-40% | Benchmark suite |
| Compilation time increase | < 2x | Time full build |
| Dead code eliminated | > 25% | Analyze output |

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Compilation becomes too slow | Incremental compilation, caching |
| Debugging becomes difficult | Source maps, debug mode without CTE |
| Too much specialization | Heuristics to limit duplication |
| Incompatible with existing code | Opt-in via @ annotations |

## Alternatives Considered

1. **Runtime JIT**: Not feasible on Z80
2. **Link-Time Optimization**: Limited without source info
3. **Profile-Guided Optimization**: Requires runtime profiling
4. **Manual Specialization**: Too much developer burden

## Implementation Priority

1. ⚡ **@execute** - Immediate value, foundation for others
2. 🎯 **@specialize** - High impact on performance
3. 🔒 **@proof** - Critical for safety-critical code
4. 🤖 **@derive** - Developer productivity boost
5. 📊 **@analyze_usage** - Advanced optimization
6. 🎪 **@compile_time_vtable** - Ultimate optimization

## Decision Outcome

**APPROVED** - This is the natural evolution of MinZ's compile-time philosophy. It transforms interfaces from "zero-cost" to "negative-cost" abstractions, perfectly aligned with our goal of modern abstractions on vintage hardware.

## References

- [Cast Interface Implementation (ADR-001)](./ADR_001_Cast_Interface.md)
- [TSMC Philosophy](./145_TSMC_Complete_Philosophy.md)
- [Swift Compile-Time Optimization](https://swift.org/blog/whole-module-optimizations/)
- [Rust Const Evaluation](https://doc.rust-lang.org/reference/const_eval.html)

---

*"From zero-cost to negative-cost: interfaces that optimize themselves out of existence."*