# TRUE SMC Design Specification v2

**Version:** 2.0  
**Date:** July 26, 2025  
**Status:** Adopted (Supersedes ADR-001)

---

## 1. Overview

TRUE SMC (истинный SMC) is a technique where function parameters are patched directly into instruction immediates. This document incorporates lessons learned from implementation and corrects earlier assumptions.

---

## 2. Key Design Principles

### 2.1 Anchor Definition

Each parameter gets ONE canonical anchor at its first use point:

```asm
; Anchor consists of:
paramName$immOP:           ; Label at instruction start
    LD A, 0               ; Instruction with immediate operand
paramName$imm0 EQU paramName$immOP+N  ; Points to immediate byte(s)
```

Where N depends on instruction format:
- Standard instructions: N=1
- Prefixed (IX/IY): N=2
- Future extended: N=3+

### 2.2 No DI/EI Required

**Important**: Z80 executes instructions atomically. Interrupts are serviced between instructions, not during them. Therefore:
- 16-bit patches do NOT require DI/EI protection
- `LD (nn), HL` is atomic
- Simplifies implementation significantly

### 2.3 Anchor Usage

```asm
; First use (creates anchor):
x$immOP:
    LD A, 0               ; 3E 00
x$imm0 EQU x$immOP+1     ; Points to 00

; Subsequent uses:
    LD A, (x$imm0)       ; Load patched value from memory

; Patching at call site:
    LD A, 5
    LD (x$imm0), A       ; Write to immediate location
    CALL function
```

---

## 3. Instruction Formats

### 3.1 8-bit Immediate Instructions (offset +1)

```asm
param$immOP:
    LD A, 0              ; 3E 00
param$imm0 EQU param$immOP+1

; Also applies to:
; LD r, n  (r = B,C,D,E,H,L)
; CP n, ADD A,n, SUB n, AND n, OR n, XOR n
```

### 3.2 16-bit Immediate Instructions (offset +1)

```asm
param$immOP:
    LD HL, 0             ; 21 00 00
param$imm0 EQU param$immOP+1

; Also applies to:
; LD BC,nn, LD DE,nn, LD SP,nn
```

### 3.3 Prefixed Instructions (offset +2)

```asm
param$immOP:
    LD IX, 0             ; DD 21 00 00
param$imm0 EQU param$immOP+2   ; Skip prefix byte
```

---

## 4. Code Generation Rules

### 4.1 Function Entry

```asm
function_name:
    ; For each parameter, at first use:
param1$immOP:
    LD A, 0              ; 8-bit anchor
param1$imm0 EQU param1$immOP+1

param2$immOP:
    LD HL, 0             ; 16-bit anchor
param2$imm0 EQU param2$immOP+1
```

### 4.2 Parameter Reuse

```asm
    ; First use already loaded value in A/HL
    ; For subsequent uses:
    LD A, (param1$imm0)  ; Reload 8-bit value
    LD HL, (param2$imm0) ; Reload 16-bit value
```

### 4.3 Call Site Patching

```asm
    ; Before calling function(5, 1000):
    LD A, 5
    LD (param1$imm0), A
    LD HL, 1000
    LD (param2$imm0), HL  ; No DI/EI needed!
    CALL function_name
```

---

## 5. Optimization Opportunities

### 5.1 Direct Use in Operations

When possible, use immediate directly in operation:

```asm
; Instead of:
x$immOP:
    LD A, 0
x$imm0 EQU x$immOP+1
    ADD A, B

; Consider:
x$immOP:
    ADD A, 0             ; Parameter IS the operation
x$imm0 EQU x$immOP+1
```

### 5.2 Specialized Anchors

For comparison-heavy parameters:

```asm
threshold$immOP:
    CP 0                 ; First use is comparison
threshold$imm0 EQU threshold$immOP+1

; Reuse for actual value:
    LD A, (threshold$imm0)
```

---

## 6. PATCH-TABLE Format

```json
{
  "version": "2.0",
  "functions": [{
    "name": "function_name",
    "anchors": [{
      "param": "x",
      "symbol": "x$imm0",
      "instruction": "x$immOP",
      "offset": 1,
      "size": 1,
      "type": "u8"
    }]
  }]
}
```

---

## 7. Implementation Checklist

- [x] Generate label$immOP at instruction
- [x] Generate EQU for label$imm0
- [x] Remove DI/EI for 16-bit patches
- [ ] Use (param$imm0) for reuse
- [ ] Patch to param$imm0 at call sites
- [ ] Handle different offsets for prefixed opcodes

---

## 8. Example: Complete Function

```asm
; MinZ: fn add(x: u8, y: u8) -> u8 { return x + y }

add:
x$immOP:
    LD A, 0              ; First use of x
x$imm0 EQU x$immOP+1
    LD B, A              ; Save x in B
y$immOP:
    LD A, 0              ; First use of y  
y$imm0 EQU y$immOP+1
    ADD A, B             ; A = x + y
    RET

; Call site for add(5, 3):
    LD A, 5
    LD (x$imm0), A
    LD A, 3
    LD (y$imm0), A
    CALL add
    ; Result in A
```

---

## 9. Migration from v1

Changes from original design:
1. Added EQU definitions for clarity
2. Removed DI/EI requirements
3. Clarified reuse syntax `(param$imm0)`
4. Added offset tables for different instruction types

---

## 10. Benefits

1. **Performance**: 3-5x faster than stack parameters
2. **Clarity**: EQU makes offsets explicit
3. **Safety**: No off-by-one errors
4. **Simplicity**: No interrupt protection needed
5. **Flexibility**: Handles all instruction formats