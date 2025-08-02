# MinZ Future Roadmap: Aligned with Current Compiler State

**Version**: 0.2.1  
**Date**: July 26, 2025  
**Status**: Planning Document

---

## Overview

This roadmap clarifies what features align with the current MinZ compiler implementation (v0.2.0) and what should be postponed. It emphasizes Z80-native patterns, particularly for error handling using the Carry flag instead of heap-allocated error tuples.

---

## 🟢 Currently Implemented (v0.2.0)

### Core Language Features
- ✅ Module imports (`import zx.screen`)
- ✅ Basic types (u8, u16, i8, i16, bool)
- ✅ Arrays with element assignment (`arr[i] = value`)
- ✅ String literals
- ✅ Structs and enums
- ✅ While loops
- ✅ Function calls with type checking
- ✅ Module constants (`screen.BLACK`)

### Compiler Infrastructure
- ✅ SMC-style fixed memory slots (not true anchors yet)
- ✅ Tree-sitter and fallback parsers
- ✅ Basic type inference
- ✅ Z80 code generation

---

## 🟡 Phase 0.3: Z80-Native Error Handling

### Error Handling via Carry Flag (Z80 Way)

Instead of Elixir-style tuples `{:ok, value}` / `{:error, code}`, use Z80's native Carry flag:

```minz
enum Error {
    DivisionByZero = 1,
    InvalidInput = 2,
    OutOfBounds = 3
}

fun divide?(a: u16, b: u16) -> u16 ? Error {
    if b == 0 {
        @error(Error.DivisionByZero)  // Sets CY=1, A=error code
    }
    return a / b  // CY=0 on success
}

// Usage with ? operator
result = divide?(10, 2) ? onerror
    when Error.DivisionByZero
        print("Cannot divide by zero")
    else
        @error()  // Propagate error
enderror
```

**Z80 Implementation**:
```asm
divide:
    ; Check for zero
    LD A, D
    OR E
    JR NZ, .not_zero
    
    ; Error path
    LD A, 1          ; Error.DivisionByZero
    SCF              ; Set Carry flag
    RET
    
.not_zero:
    ; Success path
    CALL do_division
    OR A             ; Clear Carry flag
    RET
```

**Why This is Better for Z80**:
- Single flag check (1-2 cycles) vs tuple unpacking
- Error code fits in A register
- Natural integration with Z80 conditional jumps
- No heap allocation or register pair waste
- Matches hardware error patterns (CY for overflow, etc.)

### Return Value Alias ($)

```minz
fun add(a: i16, b: i16) -> i16 {
    $ = a + b  // $ is alias for return value
}
// Equivalent to: return a + b
```

This aligns with SMC patterns where return values have fixed locations.

---

## 🟡 Phase 0.4: TRUE SMC Implementation

### Real SMC Anchors (Not Just Fixed Slots)

Current compiler uses fixed addresses like `$F000`. True SMC patches immediates:

```minz
@abi(smc)
fun draw(x: u8, y: u8, color: u8) {
    LD A, 0      ; x$imm0 - will be patched to actual x value
    OUT (FE), A
    
    LD A, (x$imm0)  ; Reuse x by reading from anchor address
}
```

**Implementation Steps**:
1. CFG analysis for anchor placement
2. PATCH-TABLE generation
3. Immediate patching logic
4. SMC undo-log for recursion

### Function Argument Propagation

```minz
fun parent(x: u8) {
    child(x)  // Read from parent's x$imm0, patch child's x$imm0
}
```

---

## 🟡 Phase 0.5: Language Ergonomics (Zero-Cost)

### Named Parameters
```minz
draw(x: 10, y: 20, color: 7)  // Pure syntax sugar
```

### For Loops with DJNZ
```minz
for i in 10..0 {    // Compiles to DJNZ
    plot(i, y)
}
```

### Defer Blocks
```minz
fun with_border() {
    screen.set_border(1)
    defer { screen.set_border(0) }  // Guaranteed cleanup
    // ...
}
```

### Match on Constants
```minz
match key {
    KEY_UP => move_up(),
    KEY_DOWN => move_down(),
    _ => {}
}
```

---

## 🔴 Phase 0.6+: Postponed Features

### Why Postpone These?

These features require significant runtime support or don't align with Z80 architecture:

### 1. Heap-Based Error Tuples ❌
- **Problem**: Requires heap allocation, register pairs
- **Solution**: Use Carry flag approach (Phase 0.3)

### 2. Dynamic Closures ❌
- **Problem**: Requires heap, captured variables
- **Solution**: Use `smc_bind` for compile-time specialization

### 3. Goroutines/Channels (Full Implementation) ❌
- **Problem**: Stack switching, scheduler overhead
- **Solution**: Simple state machines with `yield` (later)

### 4. Generic Types ❌
- **Problem**: Type erasure or code bloat
- **Solution**: Compile-time specialization via meta functions

### 5. Reflection/RTTI ❌
- **Problem**: Runtime type information overhead
- **Solution**: Compile-time only via meta programming

---

## 📊 Implementation Priority Matrix

| Feature | Complexity | Z80 Fit | User Value | Priority |
|---------|------------|---------|------------|----------|
| CY-flag errors | Low | Perfect | High | **0.3** |
| TRUE SMC anchors | High | Perfect | High | **0.4** |
| Named params | Low | Perfect | Medium | **0.5** |
| DJNZ loops | Low | Perfect | High | **0.5** |
| Defer blocks | Medium | Good | Medium | **0.5** |
| Match constants | Medium | Good | Medium | **0.5** |
| Function pointers | High | Good | Medium | **0.6** |
| smc_bind | High | Perfect | High | **0.6** |
| Meta functions | Very High | Good | Low | **0.7** |
| State machines | High | Good | Low | **0.8** |

---

## 🎯 Success Metrics

### Phase 0.3 (Error Handling)
- All examples use CY-flag errors
- Zero heap allocations for errors
- Benchmarks show 5-10x faster than tuple approach

### Phase 0.4 (TRUE SMC)
- MNIST example runs 2-3x faster
- Recursive functions work with SMC
- Clear PATCH-TABLE documentation

### Phase 0.5 (Ergonomics)  
- Code readability improves 50%
- Zero runtime overhead verified
- All examples updated to new syntax

---

## 🚫 Anti-Patterns to Avoid

1. **C-style thinking**: Stack frames, heap allocation
2. **High-level abstractions**: That hide Z80 reality
3. **Runtime polymorphism**: Static dispatch only
4. **Hidden costs**: Every feature must have predictable performance

---

## 📅 Timeline Estimate

- **Phase 0.3**: 2-3 weeks (Error handling + core fixes)
- **Phase 0.4**: 4-6 weeks (TRUE SMC implementation)
- **Phase 0.5**: 3-4 weeks (Language ergonomics)
- **Phase 0.6+**: Evaluate based on community needs

---

## 🔧 Current Compiler Alignment

### What Works Today
- Basic SMC slots (fixed addresses)
- Module system fundamentals
- Type checking infrastructure
- Code generation pipeline

### What Needs Minor Changes
- Error propagation (add CY flag handling)
- Return value aliasing ($)
- For loop desugaring

### What Needs Major Work
- TRUE SMC with anchor patching
- Function pointer infrastructure
- Meta programming system

---

## 💡 Key Insight: Z80 First, Language Second

Every feature must answer:
1. How does this map to Z80 idioms?
2. What's the T-state cost?
3. Can we do it at compile time?
4. Does it make SMC better or worse?

If the answer to any is negative, the feature gets redesigned or dropped.

---

## 🏁 Next Steps

1. **Implement CY-flag error handling** (Phase 0.3)
2. **Update all examples** to use new error pattern
3. **Begin TRUE SMC design** documents
4. **Community feedback** on roadmap priorities

The goal is a language that feels modern but generates code a 1980s assembly programmer would write by hand - just better and faster.