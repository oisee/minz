# ADR-001: Function Pointers - Parked in Favor of Compile-Time Alternatives

**Status:** Accepted
**Date:** 2025-12-17
**Deciders:** MinZ Core Team

## Context

MinZ targets vintage hardware (Z80, 6502) where every byte and cycle counts. The question arose whether to implement runtime function pointers (`fn(u8) -> u8` type parameters).

### The Problem
```minz
fun apply(f: fn(u8) -> u8, x: u8) -> u8 {
    return f(x);  // How to implement this call?
}

fun main() -> void {
    let result = apply(double, 5);  // Pass function by name
}
```

## Decision

**PARKED** - Do not implement runtime function pointers. Use compile-time alternatives instead.

## Rationale

### Runtime Function Pointers on Z80 (Rejected)

**Implementation would require:**
```asm
; Store function address
LD HL, double_addr
LD (fn_ptr), HL

; Indirect call
LD HL, (fn_ptr)
JP (HL)           ; or CALL wrapper
```

**Problems:**
1. **Overhead:** 16-bit pointer storage + indirect call (vs direct `CALL label`)
2. **No inlining:** Compiler can't optimize through pointer
3. **Complex ABI:** Need calling convention for indirect calls
4. **Register pressure:** HL tied up for address

### Compile-Time Alternatives (Preferred)

#### 1. Zero-Cost Lambdas (Already Implemented!)
```minz
numbers.iter()
    .map(|x| x * 2)      // Lambda → direct CALL
    .filter(|x| x > 5)   // No pointer overhead!
    .forEach(|x| print(x));
```
Lambdas compile to named functions with direct calls.

#### 2. CTIE (Compile-Time Execution)
```minz
@ctie
fun apply(f: fn(u8) -> u8, x: u8) -> u8 {
    return f(x);
}
// Entire operation computed at compile time!
```

#### 3. Future: Monomorphization
```minz
fun apply[F](f: F, x: u8) -> u8 where F: fn(u8) -> u8 {
    return f(x);
}
// Compiler generates specialized: apply_double(5)
```

#### 4. Future: Interface Dispatch (Compile-Time Resolved)
```minz
interface Mapper { fun map(x: u8) -> u8; }
// When concrete type known → direct CALL (no vtable)
```

## Consequences

### Positive
- Zero overhead for higher-order patterns (lambdas work great)
- Simpler compiler implementation
- Better optimization opportunities
- True to "zero-cost abstractions" philosophy

### Negative
- Can't pass functions dynamically at runtime
- No callback patterns with unknown functions
- Less flexible than full function pointers

### Acceptable Trade-offs
- For truly dynamic dispatch, use `case` with known function set
- Most iterator/functional patterns covered by lambdas
- Retro hardware rarely needs runtime polymorphism

## Future Considerations

If function passing becomes essential, implement as:
1. **Address introspection:** `@addr(function_name)` for raw address
2. **ABI matching:** Compile-time verification of signatures
3. **Monomorphization:** Generic functions specialized per call site

## Related
- P2 #9: Iterator DJNZ Optimization (uses lambdas)
- docs/141_Lambda_Iterator_Revolution_Complete.md
- Zero-cost lambda implementation in semantic analyzer

---

*MinZ Philosophy: Compile-time is always better than runtime on vintage hardware.*
