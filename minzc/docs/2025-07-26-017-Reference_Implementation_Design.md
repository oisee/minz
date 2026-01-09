# 037: SMC-Powered References Implementation Design

## Overview

This document outlines the implementation of references in MinZ using our TRUE SMC optimization technique. References provide safe, zero-overhead access to values without the complexity of pointers.

## Syntax Design

```minz
// Reference parameter
fun increment(ref x: u8) -> void {
    x = x + 1;
}

// Reference in struct
type Node = struct {
    value: u8,
    next: ref Node  // Reference to another node
};

// Taking a reference
fun main() -> void {
    let mut value: u8 = 10;
    increment(ref value);  // Pass by reference
    
    let r: ref u8 = ref value;  // Store reference
}
```

## Implementation Strategy

### 1. Reference as SMC Address Patch

Unlike regular SMC that patches VALUES, references patch ADDRESSES:

```minz
fun modify(ref x: u8) -> void {
    x = 42;
}
```

Compiles to:
```asm
modify:
    LD A, 42
x_ref$imm:
    LD ($0000), A    ; Address patched here!
    RET
```

Call site:
```asm
; Get address of variable
LD HL, variable_addr
; Patch it into the function
LD (x_ref$imm), HL
CALL modify
```

### 2. Reference Reading

```minz
fun read_ref(ref x: u8) -> u8 {
    return x;
}
```

Compiles to:
```asm
read_ref:
x_read$imm:
    LD A, ($0000)    ; Address patched
    RET
```

### 3. Multiple Reference Access

For multiple accesses to same reference, patch once:

```minz
fun swap_add(ref a: u8, ref b: u8) -> void {
    let temp: u8 = a;
    a = b;
    b = temp + 1;
}
```

Compiles to:
```asm
swap_add:
a_ref$imm:
    LD A, ($0000)    ; Read a
    LD B, A          ; Save temp
b_ref$imm:
    LD A, ($0000)    ; Read b
a_store$imm:
    LD ($0000), A    ; Store to a (same address as a_ref$imm)
    LD A, B
    INC A
b_store$imm:
    LD ($0000), A    ; Store to b (same address as b_ref$imm)
    RET
```

## AST Changes

Add to ast.go:
```go
// RefType represents a reference type
type RefType struct {
    BaseType Type
    StartPos Position
    EndPos   Position
}

// RefExpr represents taking a reference
type RefExpr struct {
    Expr     Expression
    StartPos Position
    EndPos   Position
}
```

## IR Changes

Add to ir.go:
```go
// New opcodes
OpLoadRef      // Load through reference
OpStoreRef     // Store through reference
OpTakeRef      // Get address of variable

// RefType represents reference types
type RefType struct {
    Base Type
}
```

## Semantic Analysis

1. **Reference Taking**: Only allowed on:
   - Local variables
   - Function parameters
   - Struct fields
   - Array elements (compile-time index)

2. **Lifetime Checking**: Simple rule - reference cannot outlive referent

3. **Mutability**: References inherit mutability from target

## Code Generation

### Taking a Reference
```minz
let r: ref u8 = ref value;
```
Generates:
```asm
LD HL, value_addr
LD (r_storage), HL
```

### Passing References to TRUE SMC Functions
```minz
modify(ref value);
```
Generates:
```asm
LD HL, value_addr
LD (x_ref$imm), HL    ; Patch address into function
CALL modify
```

## Advantages Over Pointers

1. **No Null**: References must always be valid
2. **No Arithmetic**: Can't accidentally corrupt
3. **Zero Overhead**: Direct addressing via SMC
4. **Type Safe**: Know exactly what's referenced
5. **Optimization Friendly**: Compiler knows aliasing

## Implementation Phases

### Phase 1: Basic References (1 week)
- Reference parameters only
- Single read/write per function
- No reference storage

### Phase 2: Full References (2 weeks)
- Reference variables
- Multiple access optimization
- Struct fields as references

### Phase 3: Advanced Features (1 week)
- Array element references (constant index)
- Reference lifetime validation
- Optimization pass

## Example: Linked List with References

```minz
type Node = struct {
    value: u8,
    has_next: bool,
    next_addr: u16  // Address of next node
};

fun traverse(ref node: Node) -> u8 {
    let mut sum: u8 = node.value;
    
    if node.has_next {
        // Manual "dereference" for now
        let next: ref Node = @ref_from_addr(node.next_addr);
        sum = sum + traverse(next);
    }
    
    return sum;
}
```

## Conclusion

SMC-powered references give MinZ a unique advantage:
- Safer than pointers
- Faster than traditional references
- Perfect fit for Z80 architecture

This design maintains MinZ's philosophy: modern safety with vintage performance.