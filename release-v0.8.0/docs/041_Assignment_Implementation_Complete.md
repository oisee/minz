# Assignment Implementation Complete

## Overview

Successfully implemented assignment statements in MinZ, enabling mutable variables and TSMC reference updates. This was a critical milestone for making MinZ a practical language.

## Key Achievements

### 1. Grammar and Parsing
- Added assignment (`=`) as a binary expression operator in `grammar.js`
- Implemented operator extraction in S-expression parser by reading source code positions
- Fixed variable declaration parsing to detect `var`/`let` keywords for mutability

### 2. Semantic Analysis
- Implemented `analyzeAssignment` function that:
  - Checks variable mutability (preventing assignment to `let` bindings)
  - Detects TSMC reference parameters and generates `OpStoreTSMCRef`
  - Handles regular variables with `OpStoreVar`
- Fixed duplicate expression evaluation by handling assignment before analyzing operands

### 3. Code Generation
- Enhanced `OpStoreVar` and `OpLoadVar` to handle different types correctly:
  - u8/i8 values use A register (single byte operations)
  - u16/i16 values use HL register (word operations)
- Implemented TSMC reference updates that modify immediate operands directly

### 4. TSMC Reference Example

Created working example demonstrating TRUE self-modifying code:

```minz
@abi("smc")
fun strlen_tsmc(str: *u8) -> u16 {
    var len: u16 = 0;
    
    while *str != 0 {
        len = len + 1;
        str = str + 1;  // This modifies the immediate in the dereference above!
    }
    
    return len;
}
```

Generated assembly shows the TSMC pattern:
```asm
str$immOP:
    LD HL, 0000      ; TSMC ref address for str
str$imm0 EQU str$immOP+1
...
LD (str$imm0), HL    ; Update TSMC reference immediate
```

## Technical Details

### Assignment Flow
1. Parser detects `=` operator in binary expression
2. Semantic analyzer routes to `analyzeAssignment` instead of normal binary op
3. Assignment checks mutability and variable type
4. For TSMC refs: generates `OpStoreTSMCRef` to modify immediate
5. For regular vars: generates `OpStoreVar` with type-aware register usage

### Type-Aware Code Generation
- Locals track their type in `ir.Local` struct
- Store/Load operations check type to use correct instructions:
  - u8: `LD A, ($addr)` / `LD ($addr), A`
  - u16: `LD HL, ($addr)` / `LD ($addr), HL`

## Impact

This implementation enables:
- Mutable state in functions
- Loop counters and accumulators
- TSMC self-modifying patterns
- Foundation for more complex features

## Next Steps

With assignments working, we can now focus on:
1. More complex assignment targets (array elements, struct fields)
2. Compound assignments (`+=`, `-=`, etc.)
3. Auto-dereferencing in common contexts
4. Advanced TSMC patterns and optimizations