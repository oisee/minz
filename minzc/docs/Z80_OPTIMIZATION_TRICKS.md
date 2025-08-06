# Z80 Assembly Optimization Tricks and Patterns

This document compiles various Z80 assembly optimization patterns and tricks for efficient code generation.

## 16-bit Comparison Techniques

### 1. The Classic SBC/ADD Trick
The most versatile method for comparing two 16-bit registers while preserving values:

```z80
; Compare HL with DE
OR A          ; Reset carry flag
SBC HL, DE    ; Subtract DE from HL with carry
ADD HL, DE    ; Add DE back to restore HL
```

Flag results:
- If HL = DE: Z=1, C=0
- If HL < DE: Z=0, C=1  
- If HL > DE: Z=0, C=0

**Note**: If carry is already clear, omit the `OR A` to save a byte.

### 2. Direct Comparison (Destructive)
When you don't need to preserve HL:

```z80
; Compare HL with DE (destroys HL)
OR A          ; Clear carry
SBC HL, DE    ; HL = HL - DE
```

### 3. Byte-by-Byte Comparison
For comparing with constants:

```z80
; Compare HL with #B2C0
LD A, H
CP #B2        ; Compare high byte
RET NZ        ; Return if not equal
LD A, L
CP #C0        ; Compare low byte
```

### 4. Signed 16-bit Comparisons
For signed comparisons, check sign bits first:

```z80
cmpgte:
    LD A, H
    XOR D
    JP M, cmpgte_signs_differ
    SBC HL, DE
    JR NC, cmpgte_true
    ; ... handle false case
```

## Common Peephole Optimization Patterns

### 1. Redundant Stack Operations
```z80
; Before:
POP HL
PUSH HL      ; Redundant - can be eliminated

; After:
; (removed)
```

### 2. Load Zero Optimization
```z80
; Before:
LD A, 0

; After:
XOR A        ; Smaller and sets flags
```

### 3. Increment/Decrement vs Add/Sub
```z80
; Before:
ADD A, 1

; After:
INC A        ; Smaller and faster
```

### 4. Double Register Loads
```z80
; Before:
LD H, D
LD L, E

; After:
EX DE, HL    ; If swapping is acceptable
```

### 5. Immediate Load Optimization
```z80
; Before:
LD HL, #0000
LD D, H
LD E, L
EX DE, HL

; After:
LD DE, #0000  ; Load directly to target
```

### 6. Stack Manipulation
```z80
; Dropping 2 bytes from stack
; Before:
POP DE        ; Wastes result

; After:
INC SP
INC SP

; For larger drops (16 bytes):
LD HL, 16
ADD HL, SP
LD SP, HL
```

## Z80-Specific Tricks

### 1. 256-byte Aligned Tables
```z80
; Table at #XX00
LD H, table_high_byte
LD L, index        ; Fast table[index] access
LD A, (HL)
```

### 2. Self-Modifying Code (SMC)
```z80
LD (savemask), A
; ... code ...
savemask = $+1
LD A, #00         ; #00 is placeholder, modified at runtime
```

### 3. Byte Stuffing
```z80
JR NC, foo
LD A, #44
DB #26            ; LD L,# opcode
foo:
LD A, #40         ; Becomes operand for LD L if jump not taken
```

### 4. DJNZ for Loops
```z80
; Instead of:
DEC C
JP NZ, loop

; Use:
DJNZ loop         ; If B register available
```

### 5. Shadow Registers
```z80
; For interrupt handlers or fast context switch
EXX               ; Swap BC, DE, HL with shadows
EX AF, AF'        ; Swap AF with shadow
```

## Performance Guidelines

### Register Usage
- **8-bit operations** are faster than 16-bit
- **IX/IY** instructions are slower and 1 byte larger - prefer HL/DE
- **Shadow registers** save 16-50 T-states vs PUSH/POP

### Instruction Timing
```
XOR A            : 4 T-states
LD A, 0          : 7 T-states
INC A            : 4 T-states  
ADD A, 1         : 7 T-states
EX DE, HL        : 4 T-states
LD D,H; LD E,L   : 8 T-states
```

### Memory Access
- **Aligned tables**: Use H for high byte, L for index
- **LDIR vs LDI**: Unrolled LDI is faster but larger
- **Direct addressing** beats indexed when possible

## Common Instruction Sequences to Optimize

### 1. Register Transfer
```z80
; Avoid:
LD D, H
LD E, L
EX DE, HL

; Better:
EX DE, HL        ; If just swapping
```

### 2. Conditional Jumps
```z80
; Avoid:
OR A
JP Z, label
JP next
label:

; Better:
OR A
JR NZ, next      ; Use relative jumps when possible
```

### 3. Loop Counters
```z80
; For 8-bit counters:
LD B, count
loop:
    ; ... loop body ...
    DJNZ loop

; For 16-bit counters:
LD BC, count
loop:
    ; ... loop body ...
    DEC BC
    LD A, B
    OR C
    JR NZ, loop
```

## MinZ Compiler Integration

These patterns are implemented in:
- `/pkg/optimizer/assembly_peephole.go` - Assembly-level peephole patterns
- `/pkg/codegen/z80.go` - Code generation with these optimizations in mind
- `/pkg/optimizer/` - Higher-level optimization passes

The compiler automatically applies many of these optimizations when the `-O` flag is used.