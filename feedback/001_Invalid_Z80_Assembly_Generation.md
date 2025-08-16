# 001: Invalid Z80 Assembly Generation

**From**: MZA Verification Colleague  
**To**: MinZ Compiler Development Team  
**Date**: 2025-08-16  
**Priority**: 🔴 CRITICAL - Blocking all assembler compatibility

## Executive Summary

MinZ is generating **invalid Z80 assembly** that no assembler can process. This blocks 100% compatibility with SjASMPlus and other standard assemblers.

## Critical Issues Found

### 1. Invalid Shadow Register Direct Access

**❌ MinZ Currently Generates**:
```asm
LD C', A      ; NOT VALID Z80 - Can't directly access shadow registers
LD B', 42     ; INVALID - No such instruction exists
LD HL', DE    ; DOUBLY INVALID - Both syntax and instruction wrong
```

**✅ Correct Z80 Must Be**:
```asm
EXX           ; Switch to shadow register bank (BC', DE', HL')
LD C, A       ; Now operating on C' (shadow C)
LD B, 42      ; Now operating on B' (shadow B)
EXX           ; Switch back to main registers
```

**Why This Matters**: 
- Z80 has no direct shadow register access instructions
- `EXX` swaps BC↔BC', DE↔DE', HL↔HL' as a set
- `EX AF, AF'` swaps A↔A', F↔F' separately

### 2. Non-Existent Register-to-Register Moves

**❌ MinZ Currently Generates**:
```asm
LD HL, DE     ; NO SUCH Z80 INSTRUCTION EXISTS!
LD BC, HL     ; INVALID
LD DE, BC     ; INVALID
```

**✅ Correct Z80 Must Be**:
```asm
; To copy DE to HL:
LD H, D       ; Copy high byte
LD L, E       ; Copy low byte

; To copy HL to BC:
LD B, H       ; Copy high byte
LD C, L       ; Copy low byte
```

### 3. Unnecessary String Escaping

**❌ MinZ Currently Generates**:
```asm
DB "Hello\'s World"    ; Unnecessary \' escape
DB "Path\\to\\file"    ; Excessive backslash escaping
```

**✅ Should Generate**:
```asm
DB "Hello's World"     ; Single quotes need no escape in double-quoted strings
DB "Path\to\file"      ; Single backslash is fine
DB "Say \"Hello\""     ; Only escape double quotes within double quotes
```

## Impact Analysis

| Issue | Files Affected | Assemblers Blocked | Fix Complexity |
|-------|---------------|-------------------|----------------|
| Shadow register syntax | ~500+ | ALL | Medium |
| Invalid reg-to-reg moves | ~300+ | ALL | Easy |
| String over-escaping | ~100+ | SjASMPlus | Easy |

**Total Impact**: 0% of MinZ-generated assembly can be processed by standard assemblers

## Recommended Actions

### Immediate Fixes Required in MinZ Compiler

1. **Shadow Register Access**
   - Remove all `LD reg', value` generation
   - Implement proper `EXX` wrapping for shadow register access
   - Use `EX AF, AF'` for accumulator shadow access

2. **Register Pair Moves**
   - Detect `LD r16, r16` patterns
   - Expand to two 8-bit moves: `LD rh, rh : LD rl, rl`
   - Special case: Use `PUSH/POP` for HL↔IX/IY transfers

3. **String Generation**
   - Only escape `"` within double-quoted strings
   - Remove unnecessary `\'` escaping
   - Simplify backslash handling

## Proposed MZA Enhancement (Workaround)

While MinZ is being fixed, MZA can add **pseudo-instructions** to handle common patterns:

### Pseudo-Instruction Expansion Table

```asm
; Input (Pseudo)     →  Output (Real Z80)
LD BC, DE           →  LD B, D : LD C, E
LD HL, BC           →  LD H, B : LD L, C
LD DE, HL           →  LD D, H : LD E, L

; Shadow register pseudo-ops
LD BC', DE          →  EXX : LD B, D : LD C, E : EXX
LD HL', 1234        →  EXX : LD HL, 1234 : EXX

; Index register moves
LD IX, BC           →  LD IXH, B : LD IXL, C
LD IY, DE           →  LD IYH, D : LD IYL, E
LD IX, HL           →  PUSH HL : POP IX    ; Stack transfer
```

## Benefits of Fixing

1. **Immediate Compatibility**: MinZ output works with ALL Z80 assemblers
2. **Reduced Confusion**: Developers see valid Z80, not invented syntax
3. **Better Debugging**: Any Z80 tool can process the output
4. **Standards Compliance**: Follows Zilog Z80 specification

## Test Case

Create `test_valid_z80.minz`:
```minz
global shadow_test: u8 = 0;

fun main() -> void {
    // Test shadow registers
    let a = 42;
    // Should NOT generate: LD C', 42
    // Should generate: EXX : LD C, 42 : EXX
}
```

Expected output should assemble with:
- ✅ MZA
- ✅ SjASMPlus  
- ✅ z80asm
- ✅ Any standard Z80 assembler

## Conclusion

The current invalid assembly generation is the **#1 blocker** for assembler compatibility. These are not edge cases - they affect core functionality. Fixing these issues will immediately improve compatibility from 0% to potentially 70%+.

**Recommended Priority**: Fix register-to-register moves first (easiest), then shadow registers, then string escaping.

---

*Note: This feedback is part of the MZA Verification Project to achieve 100% assembler compatibility.*