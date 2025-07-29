# Auto-Dereferencing Design for MinZ

## Overview

Auto-dereferencing makes pointer usage more ergonomic by automatically inserting dereference operations in certain contexts. This is especially important for TSMC references where we want clean, readable code.

## Design Principles

1. **Predictable**: Auto-deref should follow clear, consistent rules
2. **TSMC-Friendly**: Should work seamlessly with TSMC references
3. **Explicit Override**: Allow explicit dereferencing when needed
4. **Type Safe**: Never compromise type safety

## Auto-Deref Contexts

### 1. Assignment Target
When assigning through a pointer, auto-deref the left side:
```minz
var p: *u8 = &x;
p = 42;        // Auto-deref: *p = 42
*p = 42;       // Explicit: same effect
```

### 2. Binary Operations
When a pointer is used where a value is expected:
```minz
var p: *u16 = &counter;
p = p + 1;     // Auto-deref RHS: *p = *p + 1
```

### 3. Function Arguments
When passing pointers to functions expecting values:
```minz
fun add(a: u8, b: u8) -> u8 { return a + b; }
var p: *u8 = &x;
var q: *u8 = &y;
var sum = add(p, q);  // Auto-deref: add(*p, *q)
```

### 4. TSMC Reference Context
For TSMC references, make increment operations natural:
```minz
@abi("smc")
fun iterate(ptr: *u8) {
    ptr = ptr + 1;    // Updates the TSMC immediate
}
```

## Implementation Strategy

### Phase 1: Assignment Target Auto-Deref
1. In `analyzeAssignment`, check if LHS is an identifier
2. If identifier type is pointer, insert dereference
3. Generate appropriate IR (OpLoad before OpStore)

### Phase 2: Expression Auto-Deref  
1. In `analyzeExpression`, check context
2. If pointer used where value expected, insert OpLoad
3. Track original expression for error messages

### Phase 3: TSMC-Specific Handling
1. Detect TSMC reference parameters
2. Handle pointer arithmetic specially
3. Ensure self-modification works correctly

## Examples

### Basic Auto-Deref
```minz
fun test() {
    var x: u8 = 10;
    var p: *u8 = &x;
    
    // Auto-deref in assignment
    p = 20;           // *p = 20
    
    // Auto-deref in arithmetic  
    p = p + 1;        // *p = *p + 1
    
    // Explicit when needed
    var q: *u8 = p;   // Copy pointer
    *q = 30;          // Explicit deref
}
```

### TSMC Pattern
```minz
@abi("smc")
fun strlen(str: *u8) -> u16 {
    var len: u16 = 0;
    
    // Natural syntax with auto-deref
    while str != 0 {     // Auto-deref: *str != 0
        len = len + 1;
        str = str + 1;   // Modifies TSMC immediate
    }
    
    return len;
}
```

## Edge Cases

### Multiple Indirection
```minz
var pp: **u8;
pp = 42;      // Error: can't auto-deref **u8 to u8
*pp = 42;     // Error: can't assign u8 to *u8  
**pp = 42;    // OK: explicit double deref
```

### Address-Of
```minz
var x: u8 = 10;
var p: *u8 = x;    // Error: need &x
var p: *u8 = &x;   // OK: explicit address-of
```

## Benefits

1. **Cleaner Code**: Less syntactic noise
2. **TSMC Natural**: Makes TSMC patterns feel native
3. **Beginner Friendly**: Reduces cognitive load
4. **C-Compatible**: Similar to C's array/pointer duality

## Next Steps

1. Implement assignment target auto-deref
2. Add tests for various contexts  
3. Update examples to use auto-deref
4. Document in language reference