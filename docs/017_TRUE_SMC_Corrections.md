# TRUE SMC Implementation Corrections

**Version:** 0.2.6  
**Date:** July 26, 2025  
**Status:** Technical Corrections

---

## 1. DI/EI Not Required for 16-bit Patches

**Original assumption**: DI/EI needed for atomic 16-bit updates
**Correction**: Z80 completes entire instructions atomically

The Z80 checks for interrupts **between** instructions, not during. Therefore:
- `LD HL, nn` executes atomically
- `LD (nn), HL` executes atomically  
- No DI/EI protection needed for patching

This simplifies the patching code:
```asm
; Before (overcautious):
    DI
    LD (anchor+1), HL
    EI

; After (correct):
    LD (anchor+1), HL    ; Already atomic
```

---

## 2. Anchor Addressing with EQU

**Better approach**: Use EQU to define immediate offset

```asm
; For 8-bit immediate instructions
x$immOP:
    LD A, 0            ; 3E 00
x$imm0  EQU x$immOP+1  ; Points to immediate byte

; For 16-bit immediate instructions  
y$immOP:
    LD HL, 0           ; 21 00 00
y$imm0  EQU y$immOP+1  ; Points to immediate word

; For IX/IY instructions (prefixed)
z$immOP:
    LD IX, 0           ; DD 21 00 00
z$imm0  EQU z$immOP+2  ; Skip prefix byte
```

This clarifies usage:
- **Patch**: `LD (x$imm0), A` - write to immediate location
- **Reuse**: `LD A, (x$imm0)` - read patched value from memory

---

## 3. Instruction Format Reference

### Standard immediates (+1):
| Instruction | Opcode | Bytes | Immediate at |
|------------|--------|-------|--------------|
| LD A, n    | 3E nn  | 2     | +1          |
| LD B, n    | 06 nn  | 2     | +1          |
| LD C, n    | 0E nn  | 2     | +1          |
| LD D, n    | 16 nn  | 2     | +1          |
| LD E, n    | 1E nn  | 2     | +1          |
| LD H, n    | 26 nn  | 2     | +1          |
| LD L, n    | 2E nn  | 2     | +1          |
| CP n       | FE nn  | 2     | +1          |
| ADD A, n   | C6 nn  | 2     | +1          |
| SUB n      | D6 nn  | 2     | +1          |
| AND n      | E6 nn  | 2     | +1          |
| OR n       | F6 nn  | 2     | +1          |
| XOR n      | EE nn  | 2     | +1          |

### 16-bit immediates (+1):
| Instruction | Opcode    | Bytes | Immediate at |
|------------|-----------|-------|--------------|
| LD BC, nn  | 01 nn nn  | 3     | +1          |
| LD DE, nn  | 11 nn nn  | 3     | +1          |
| LD HL, nn  | 21 nn nn  | 3     | +1          |
| LD SP, nn  | 31 nn nn  | 3     | +1          |

### Prefixed instructions (+2):
| Instruction | Opcode       | Bytes | Immediate at |
|------------|--------------|-------|--------------|
| LD IX, nn  | DD 21 nn nn  | 4     | +2          |
| LD IY, nn  | FD 21 nn nn  | 4     | +2          |

### Extended instructions (+3):
| Instruction    | Opcode          | Bytes | Immediate at |
|----------------|-----------------|-------|--------------|
| LD A, (nn)     | 3A nn nn        | 3     | +1          |
| LD (nn), A     | 32 nn nn        | 3     | +1          |
| LD HL, (nn)    | 2A nn nn        | 3     | +1          |
| LD (nn), HL    | 22 nn nn        | 3     | +1          |
| CALL nn        | CD nn nn        | 3     | +1          |
| JP nn          | C3 nn nn        | 3     | +1          |

---

## 4. Implementation Updates Needed

### Code Generation:
```asm
; Generate anchors with EQU
fun:
x$immOP:
    LD A, 0
x$imm0  EQU x$immOP+1
y$immOP:
    LD HL, 0  
y$imm0  EQU y$immOP+1
```

### Patching:
```asm
; Direct patch without +1
    LD A, 5
    LD (x$imm0), A     ; x$imm0 already points to immediate
```

### Reuse:
```asm
; Load patched value from immediate location
    LD A, (x$imm0)     ; Read the patched immediate byte
```

---

## 5. Special Cases

### If using CP, ADD, etc. as anchors:
```asm
value$immOP:
    CP 0               ; FE 00
value$imm0  EQU value$immOP+1

; First use already compares
; Reuse needs explicit load:
    LD A, (value$imm0)
    ; Now A has the value
```

### For operations that modify A:
```asm
delta$immOP:
    ADD A, 0           ; C6 00  
delta$imm0  EQU delta$immOP+1

; If we need the delta value itself later:
    LD A, (delta$imm0)
```

---

## 6. Benefits of This Approach

1. **Clearer code**: No mental +1 arithmetic everywhere
2. **Consistent**: Always use symbol$imm0 for the immediate
3. **Flexible**: Can define different offsets for prefixed opcodes
4. **Safer**: Less chance of off-by-one errors
5. **Self-documenting**: immOP vs imm0 shows intent

---

## 7. Summary

- DI/EI is unnecessary - Z80 instructions are atomic
- Use EQU to define immediate offsets clearly
- Different instructions may have different offsets
- This approach matches Z80 assembler conventions better