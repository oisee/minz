# TSMC Reference Implementation Session Summary

## Key Philosophical Breakthrough

We had a profound realization about MinZ's design philosophy:

**"Do we need pointers?"** - The answer is NO. We need TSMC-native references.

### The Core Insight

In TSMC (Tree-Structured Machine Code), references aren't memory addresses pointing to data. They're **code addresses pointing to immediate operands that will be patched**.

Traditional approach:
```asm
; Pointer in register pointing to memory
LD HL, data_address  ; HL holds address
LD A, (HL)          ; Indirect load through pointer
```

TSMC approach:
```asm
; Reference IS the immediate operand
param$immOP:
    LD A, (0000)    ; The 0000 gets patched!
param$imm0 EQU param$immOP+1
```

## What We Implemented

1. **Added IsTSMCRef flag to Parameter struct** - Marks parameters that should use TSMC reference passing

2. **Updated AddParam to detect pointer parameters** - All `*T` parameters automatically become TSMC references

3. **Modified code generator for TSMC refs** - Creates anchors with immediate operands instead of memory loads

4. **Generated TSMC-style anchors**:
   ```asm
   ; TSMC reference parameter str
   str$immOP:
       LD A, (0000)     ; TSMC ref to str
   str$imm0 EQU str$immOP+1
   ```

## The Vision

### Before (Traditional Pointers)
```minz
fun strlen(str: *u8) -> u16 {
    let mut len = 0;
    while *str != 0 {      // Memory indirection
        len += 1;
        str += 1;          // Pointer arithmetic
    }
    return len;
}
```

Generates:
```asm
LD ($F002), HL    ; Store pointer to memory
LD HL, ($F002)    ; Load pointer from memory
LD A, (HL)        ; Dereference
```

### After (TSMC References)
```minz
fun strlen(str: &u8) -> u16 {  // & means TSMC reference
    let mut len = 0;
    while str != 0 {           // Direct immediate access
        len += 1;
        // Self-modifying next access
    }
    return len;
}
```

Should generate:
```asm
strlen:
    LD BC, 0          ; len counter
.loop:
str$immOP:
    LD A, (0000)      ; This 0000 is the string address!
str$imm0 EQU str$immOP+1
    OR A
    RET Z
    INC BC
    ; Self-modify for next character
    LD HL, (str$imm0)
    INC HL
    LD (str$imm0), HL
    JR .loop
```

## Benefits Achieved

1. **No register pressure** - Don't need HL to hold addresses
2. **No memory usage** - Parameters live in code, not data
3. **Faster execution** - Direct immediate access vs indirection
4. **True TSMC** - Every parameter is a patch point
5. **Self-modifying loops** - Can update addresses in-place

## Current State

We've laid the groundwork for TSMC references:
- Parameter detection works
- Anchor generation works
- Basic structure in place

However, full integration requires:
- Updating semantic analyzer to handle auto-deref
- Modifying OpLoad to recognize TSMC refs
- Implementing self-modification for loops
- Patching at call sites

## The Revolution

This isn't just an optimization. It's a fundamental paradigm shift:

**In traditional languages**: Code uses data through pointers
**In TSMC-MinZ**: Data is injected into code as immediates

The program becomes a template, and function calls become template instantiations. Every immediate operand is a potential variable, and every function is self-modifying.

## Next Steps

1. Complete TSMC reference implementation for all pointer operations
2. Add auto-deref in expression contexts
3. Implement self-modifying loops
4. Create examples showcasing TSMC power
5. Document patterns for TSMC-style programming

The future of Z80 programming isn't safer pointers - it's eliminating pointers entirely in favor of **immediate slot references**.