# MZA Critical Finding: Forward References Not Supported

## Executive Summary
**All Phase 1 JR instructions ARE implemented** (JR NZ/Z/NC/C), but MZA is a **single-pass assembler** that doesn't support forward references. This severely limits its usefulness.

## Test Results

### ✅ What Works
```asm
start:
    NOP
    JR Z, start    ; ✅ Backward reference works
    JR NZ, start   ; ✅ Works
    JR C, start    ; ✅ Works  
    JR NC, start   ; ✅ Works
    DJNZ start     ; ✅ Works
```

### ❌ What Doesn't Work
```asm
    JR Z, skip     ; ❌ Forward reference fails
    NOP
skip:
    HALT
```

Error: `undefined symbol: skip`

## Verification
```bash
# This works (backward reference)
echo 'ORG $8000
loop: DEC A
JR NZ, loop
END' | mza - -o test.bin

# This fails (forward reference)  
echo 'ORG $8000
JR NZ, skip
skip: HALT
END' | mza - -o test.bin
# Error: undefined symbol: skip
```

## Impact Analysis

### Severity: HIGH
- **Most assembly code requires forward references**
- Loop exits often jump forward
- Conditional code blocks jump over sections
- This makes MZA unsuitable for real assembly programs

### Current Capability
- Can only jump to previously defined labels
- Forces unnatural code organization
- Cannot assemble most real-world Z80 programs

## Solution Required
MZA needs to be converted to a **two-pass assembler**:

### Pass 1: Symbol Collection
- Parse all labels and record addresses
- Build symbol table

### Pass 2: Code Generation  
- Resolve all references using symbol table
- Generate machine code

## Immediate Workaround
For MinZ compiler output:
- Use absolute jumps (JP) instead of relative (JR)
- JP supports forward references in MZA
- Less efficient (3 bytes vs 2) but functional

## Recommendations
1. **Short term**: Use JP instead of JR in MinZ codegen
2. **Medium term**: Implement two-pass assembly in MZA
3. **Long term**: Full multi-pass with optimization

## Current State
- **Instruction coverage**: 100% of Phase 1 instructions implemented
- **Usability**: 30% due to forward reference limitation
- **Priority**: Critical - blocks real program assembly