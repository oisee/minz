# 032: Architecture Decision Records (ADRs)

## ADR-001: TRUE SMC as Default Optimization

**Status**: Accepted and Implemented

**Context**: Z80 programs often need to pass parameters efficiently. Traditional stack-based parameter passing is slow.

**Decision**: Make TRUE SMC (immediate operand patching) the default calling convention for all functions.

**Consequences**:
- ✅ 3-4x faster parameter passing
- ✅ Enables efficient closures
- ❌ Code must be in RAM
- ❌ Harder to debug

**Notes**: This is MinZ's killer feature - embrace it fully.

---

## ADR-002: Bit Structs as First-Class Types

**Status**: Accepted and Implemented

**Context**: Hardware register manipulation requires extensive bit twiddling in traditional languages.

**Decision**: Introduce `bits` types that compile to efficient AND/OR/shift operations.

**Consequences**:
- ✅ Zero-cost hardware abstraction
- ✅ Type-safe bit manipulation
- ✅ Cleaner code for hardware
- ❌ New concept to learn

---

## ADR-003: Simple Parser vs Tree-sitter

**Status**: Under Review

**Context**: Started with tree-sitter but it had issues. Wrote simple parser but it has bugs with multi-line parsing.

**Options**:
1. Fix simple parser
2. Return to tree-sitter
3. Write new recursive descent parser

**Recommendation**: Fix simple parser first (2-3 days), then evaluate.

---

## ADR-004: Module System Design

**Status**: Proposed

**Context**: Need code organization without complex dependency management.

**Decision**: Use simple name prefixing: `module.function` becomes `module_function` internally.

**Consequences**:
- ✅ Simple to implement
- ✅ No runtime overhead
- ✅ Clear in assembly output
- ❌ No real namespaces
- ❌ Potential name conflicts

---

## ADR-005: Memory Management Philosophy

**Status**: Accepted

**Context**: Z80 has 64KB address space. Modern memory management is impossible.

**Decision**: No heap allocation. Everything is either:
- Static (compile-time known)
- Stack (automatic cleanup)
- Buffer pools (manual management)

**Consequences**:
- ✅ Predictable performance
- ✅ No GC pauses
- ✅ Fits Z80 constraints
- ❌ No dynamic data structures
- ❌ Requires different thinking

---

## ADR-006: Lambda Implementation via TRUE SMC

**Status**: Proposed

**Context**: Want modern lambda/closure support without traditional overhead.

**Decision**: Implement lambdas by copying template code and patching captured values as immediates.

**Consequences**:
- ✅ Zero-overhead closures
- ✅ No heap allocation
- ✅ Optimal performance
- ❌ Limited lambda size
- ❌ Fixed capture count

---

## ADR-007: Error Handling Strategy

**Status**: Proposed

**Context**: Need error handling without exceptions (too heavy for Z80).

**Options**:
1. Carry flag convention (like CP/M)
2. Result<T, E> types (like Rust)
3. Error codes in fixed location
4. Multiple return values

**Recommendation**: Carry flag for system calls, Result<T,E> for user code.

---

## ADR-008: Standard Library Philosophy

**Status**: Proposed

**Context**: Need useful functions without bloating programs.

**Decision**: Three-tier approach:
1. `core` - freestanding, no dependencies
2. Platform modules (`zx`, `cpm`, etc.)
3. Optional libraries (link only if used)

**Consequences**:
- ✅ Pay only for what you use
- ✅ Platform-appropriate APIs
- ✅ Small binary size
- ❌ More complex than monolithic stdlib

---

## ADR-009: Inline Assembly Integration

**Status**: Accepted

**Context**: Systems programming requires assembly for some operations.

**Decision**: Two forms:
1. Assembly blocks: `asm { ... }`
2. Assembly expressions: `asm("LD A, 0xFF") -> u8`

**Consequences**:
- ✅ Full hardware control
- ✅ Optimizer-friendly
- ✅ Type-safe integration
- ❌ Platform-specific code

---

## ADR-010: Optimization Philosophy

**Status**: Accepted

**Context**: Z80 is extremely performance-sensitive.

**Decision**: "Every cycle counts" - optimize aggressively by default:
- TRUE SMC enabled
- No bounds checking in release
- Inline aggressively
- Use self-modifying code

**Consequences**:
- ✅ Best possible performance
- ✅ Competitive with assembly
- ❌ Larger code size
- ❌ Harder debugging

---

## ADR-011: Type System Complexity

**Status**: Accepted

**Context**: Balance between expressiveness and simplicity.

**Decision**: Keep it simple:
- No generics (initially)
- No traits/interfaces
- Basic type inference
- Structural typing for compatibility

**Consequences**:
- ✅ Easy to understand
- ✅ Fast compilation
- ✅ Predictable code gen
- ❌ Some code duplication
- ❌ Less abstraction

---

## ADR-012: Build System

**Status**: Proposed

**Context**: Need to manage multi-file projects.

**Options**:
1. Simple Makefile-based
2. Custom build tool
3. Integrate with existing (cargo-like)

**Recommendation**: Start with Makefile, evolve to custom tool.

---

## Decision Framework

Every architectural decision should be evaluated against:

1. **Performance Impact**: Does it maintain zero-cost abstraction?
2. **Z80 Fit**: Does it respect platform constraints?
3. **Complexity**: Is it simple enough to understand?
4. **Value**: Does it enable better programs?
5. **Uniqueness**: Does it differentiate MinZ?

## Open Questions

1. Should we support recursive functions with TRUE SMC?
2. How to handle 16-bit operations efficiently?
3. Should we add pattern matching beyond simple enums?
4. How much compile-time evaluation is useful?
5. Should we support inline tests?

These ADRs form the philosophical foundation of MinZ - a language that embraces Z80's constraints while providing modern ergonomics.