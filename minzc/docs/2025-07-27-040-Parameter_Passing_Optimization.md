# Parameter Passing Optimizations in MinZ

## Overview

MinZ implements intelligent parameter passing that automatically chooses the most efficient method based on function characteristics and calling patterns.

## Parameter Passing Strategies

### 1. Register-Based (Fastest)
- First 2-3 parameters passed in registers
- **u8/i8**: A, E, D, C, B
- **u16/i16**: HL, DE, BC
- **Pointers**: HL (primary), DE (secondary)

### 2. Hybrid Register+Stack
- First few parameters in registers
- Remaining on stack
- Optimal for most functions

### 3. Stack-Based (Traditional)
- All parameters on stack
- Required for:
  - Variadic functions
  - Functions with many parameters (>6)
  - Recursive functions
  - Functions whose address is taken

### 4. SMC Parameter Injection
- Parameters directly patched into code
- Ultra-fast for constants
- RAM-only optimization

## Implementation Plan

### Phase 1: Basic Register Passing
```minz
// Compiler automatically uses register passing
fun add(a: u8, b: u8) -> u8 {
    return a + b;
}
// Generates:
// ; a in A, b in E
// add:
//     ADD A, E
//     RET
```

### Phase 2: Smart Allocation
```minz
fun process(ptr: *u8, len: u16, flags: u8) -> u16 {
    // ptr → HL, len → DE, flags → A
}
```

### Phase 3: Calling Convention Inference
- Analyze call sites to optimize parameter placement
- Prefer registers used by callee
- Minimize register shuffling

## Benefits

1. **2-5x faster function calls** for small functions
2. **Reduced stack usage** - important for Z80's limited stack
3. **Better register utilization** - parameters often stay in optimal registers
4. **Zero-overhead abstractions** - small functions become as fast as macros

## ABI Stability

Functions exported or with address taken always use stable stack-based ABI for compatibility.