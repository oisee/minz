# MinZ Iterator Design

## Overview

MinZ provides efficient iteration over tables (arrays) with two distinct modes optimized for different use cases on Z80 architecture.

## Iterator Modes

### 1. INTO Mode - Copy to Static Buffer

```minz
loop People into person {
    person.age += 1
    if person.active {
        process(person.name)
    }
}
```

**How it works:**
- Each element is copied to a static buffer at a fixed memory address
- Field accesses compile to direct memory addresses
- Modifications are copied back after the loop body

**Benefits:**
- Ultra-fast field access: 7 T-states (direct memory)
- No register pressure
- Can use self-modifying code optimizations
- Predictable memory layout

**Best for:**
- Read-heavy operations
- Complex computations on multiple fields
- When you need to access fields multiple times

### 2. REF TO Mode - Direct Reference

```minz
loop People ref to person {
    person.age += 1
}
```

**How it works:**
- HL register points to current element
- Field accesses use HL-relative addressing
- Direct modification of original data

**Benefits:**
- No copy overhead
- Direct modification: 11 T-states (HL+offset)
- Memory efficient
- Works with any size tables

**Best for:**
- Write-heavy operations
- Simple field updates
- Large structures where copying is expensive

## Memory Layout

### INTO Mode Memory Map

```
Static Buffer (e.g., $F000):
+0: field1
+2: field2
+4: field3
...

Compiled field access:
person.age → LD A, ($F002)  ; Direct address!
```

### REF TO Mode Access Pattern

```
HL → Current Element:
+0: field1
+2: field2  
+4: field3
...

Compiled field access:
person.age → LD A, (HL+2)   ; Relative to HL
```

## Syntax

### Basic Iteration

```minz
// Copy each element
loop table_name into var_name {
    // body
}

// Reference each element  
loop table_name ref to var_name {
    // body
}
```

### With Index

```minz
// With index counter
loop table_name indexed to var_name, index {
    print(f"Item {index}: {var_name}")
}
```

### With Condition (Future)

```minz
// Filter during iteration
loop table_name into var where var.active {
    // only active elements
}
```

## Implementation Details

### Z80 Code Generation - INTO Mode

```asm
; Setup
LD HL, table_base       ; Source pointer
LD B, table_count       ; Counter

loop_start:
    ; Copy element to buffer
    PUSH HL
    PUSH BC
    LD DE, STATIC_BUFFER
    LD BC, element_size
    LDIR
    
    ; User code with direct addresses
    LD A, (STATIC_BUFFER + age_offset)
    INC A
    LD (STATIC_BUFFER + age_offset), A
    
    ; Copy back if modified
    POP BC
    POP HL
    PUSH HL
    PUSH BC
    LD DE, STATIC_BUFFER
    EX DE, HL
    LD BC, element_size
    LDIR
    
    ; Next element
    POP BC
    POP HL
    LD DE, element_size
    ADD HL, DE
    DJNZ loop_start
```

### Z80 Code Generation - REF TO Mode

```asm
; Setup
LD HL, table_base       ; Current pointer
LD B, table_count       ; Counter

loop_start:
    ; HL points to current element
    ; Access fields with offset
    PUSH HL
    LD DE, age_offset
    ADD HL, DE
    INC (HL)            ; Modify in place
    POP HL
    
    ; Next element
    LD DE, element_size
    ADD HL, DE
    DJNZ loop_start
```

## Compiler Optimizations

### 1. Mode Selection Heuristic

The compiler can automatically choose the mode based on usage:
- More than 3 field reads → INTO mode
- Mostly writes → REF TO mode
- Mixed → Cost analysis based on field access patterns

### 2. Self-Modifying Code (SMC)

For INTO mode with compile-time constants:
```asm
smc_update:
    LD A, (BUFFER_AGE)
    ADD A, 1        ; This immediate can be SMC!
    LD (BUFFER_AGE), A
```

### 3. Register Allocation

- INTO mode: Frees up HL for other uses
- REF TO mode: Reserves HL but needs fewer memory accesses

## Performance Comparison

| Operation | INTO Mode | REF TO Mode | IX+offset |
|-----------|-----------|-------------|-----------|
| Read field | 7 T-states | 11 T-states | 19 T-states |
| Write field | 10 T-states | 11 T-states | 19 T-states |
| Setup overhead | Copy: ~21*size | None | None |

## Best Practices

1. Use INTO mode when:
   - Accessing multiple fields multiple times
   - Complex calculations on element data
   - Read-heavy operations

2. Use REF TO mode when:
   - Simple updates to one or two fields
   - Large structures (avoid copy overhead)
   - Write-heavy operations

3. Consider table layout:
   - Align frequently accessed fields early in structure
   - Group related fields for cache efficiency
   - Keep structures small when possible

## Future Extensions

1. **Parallel iteration**: Loop over multiple tables
2. **Reverse iteration**: Process backwards
3. **Step iteration**: Skip elements
4. **Early termination**: Break/continue support
5. **Nested loops**: Automatic buffer management