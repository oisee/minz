# 041: MIR to 6502 Feasibility Analysis

## Executive Summary

This document analyzes the feasibility of extending the MinZ compiler to target the MOS 6502 processor (specifically for Commodore 64). While the current compiler generates highly optimized Z80 assembly for ZX Spectrum, the intermediate representation (MIR) provides a potential abstraction layer for multi-target support. This analysis examines the architectural differences, implementation challenges, and strategic considerations for 6502 support.

**Key Finding**: While technically feasible, targeting 6502 would require significant architectural changes to the compiler, particularly in register allocation, addressing modes, and optimization strategies. The effort is estimated at 3-4 months of dedicated development.

## 1. Architecture Comparison: Z80 vs 6502

### 1.1 Register Architecture

**Z80 (Current Target)**
- **Main Registers**: A, B, C, D, E, H, L (8-bit)
- **Register Pairs**: AF, BC, DE, HL, IX, IY, SP (16-bit)
- **Shadow Registers**: Full alternate set (A', F', B', C', D', E', H', L')
- **Index Registers**: IX, IY with displacement addressing
- **Flags**: Comprehensive flag register (Z, C, S, P/V, H, N)

**6502 (Proposed Target)**
- **Registers**: A (accumulator), X, Y (index registers) - all 8-bit
- **Stack Pointer**: S (8-bit, fixed to page 1)
- **Program Counter**: PC (16-bit)
- **Flags**: P register (N, V, B, D, I, Z, C)
- **No register pairs**, no shadow registers, no 16-bit operations

### 1.2 Memory Architecture

**Z80**
- 16-bit addressing (64KB address space)
- Rich addressing modes: immediate, direct, indirect, indexed, relative
- I/O ports separate from memory space
- Bank switching common on ZX Spectrum

**6502**
- 16-bit addressing (64KB address space)
- Zero page (first 256 bytes) acts as extended register set
- Different addressing modes: zero page, absolute, indexed, indirect indexed
- Memory-mapped I/O
- Bank switching via memory mapping on C64

### 1.3 Instruction Set Differences

**Z80 Advantages**
- Native 16-bit operations (ADD HL, DE)
- Block operations (LDIR, CPIR)
- Bit manipulation instructions
- Conditional calls and returns
- EX and EXX for fast context switching

**6502 Characteristics**
- No 16-bit arithmetic (must synthesize)
- No multiply/divide (must use routines)
- Limited stack operations (only 256 bytes)
- No block operations
- Decimal mode for BCD arithmetic

## 2. MIR Analysis for Multi-Target Support

### 2.1 Current MIR Structure

The MinZ IR (MIR) currently contains Z80-specific assumptions:
```go
// From ir.go
type Z80Register uint32  // Z80-specific register definitions
type RegisterSet uint32 // Tracks Z80 registers
```

**Key MIR Operations**:
- Control flow: Jump, Call, Return
- Data movement: Load, Store, Move
- Arithmetic: Add, Sub, Mul, Div, Mod
- SMC operations: SMCLoad, SMCStore, TrueSMCPatch
- Memory: Alloc, Free, LoadPtr, StorePtr

### 2.2 Target-Agnostic Potential

Most MIR operations are conceptually portable:
- Basic arithmetic and logic operations
- Control flow structures
- Memory operations
- Function calls and returns

However, several features are Z80-optimized:
- Register tracking assumes Z80 register set
- 16-bit operations assume register pairs
- SMC operations assume Z80 instruction encoding
- Shadow register optimizations

### 2.3 Required IR Changes

To support 6502, the IR would need:

1. **Abstract Register Model**
   ```go
   type VirtualRegister interface {
       Size() int
       Kind() RegisterKind // GPR, Index, Accumulator
   }
   ```

2. **Target-Specific Annotations**
   ```go
   type Instruction struct {
       // ... existing fields
       TargetHints map[string]interface{} // Target-specific optimization hints
   }
   ```

3. **Size-Aware Operations**
   - Explicit 8-bit vs 16-bit operations
   - Multi-byte operation decomposition

## 3. Implementation Challenges

### 3.1 Register Allocation

**Z80 Current Approach**:
- Rich register set allows sophisticated allocation
- Shadow registers for interrupt handlers
- Register pairs for 16-bit operations

**6502 Challenge**:
- Only 3 registers (A, X, Y) for general use
- Heavy reliance on zero page as "registers"
- Must synthesize 16-bit operations

**Solution Strategy**:
```go
// Pseudo-code for 6502 allocator
type M6502Allocator struct {
    zeroPageVars map[ir.Register]uint8  // Map virtual regs to zero page
    accumInUse   bool
    xInUse       bool
    yInUse       bool
}
```

### 3.2 16-bit Operations

**Z80**: `ADD HL, DE` - Single instruction, 11 cycles

**6502**: Must synthesize:
```asm
; Add DE to HL equivalent (using zero page)
CLC
LDA HL_LOW
ADC DE_LOW
STA HL_LOW
LDA HL_HIGH
ADC DE_HIGH
STA HL_HIGH
; Total: ~24 cycles
```

### 3.3 Self-Modifying Code (SMC)

MinZ's revolutionary SMC optimization relies on patching immediate values in instructions.

**Z80 SMC**:
```asm
LD HL, 0000  ; 3 bytes, immediate at offset +1
```

**6502 SMC Possibilities**:
```asm
LDA #$00     ; 2 bytes, immediate at offset +1
LDX #$00     ; 2 bytes, immediate at offset +1
; But no 16-bit immediate loads!
```

**Challenge**: 6502 lacks 16-bit immediate operations, requiring different SMC strategies:
1. Use two 8-bit patches for 16-bit values
2. Use zero page pointers with SMC
3. Different optimization strategies entirely

### 3.4 Stack Operations

**Z80**: Full 16-bit stack pointer, push/pop any register pair

**6502**: 
- 8-bit stack pointer (256 bytes max)
- Can only push/pull A, X, Y, and P
- Must manually manage 16-bit values

### 3.5 Function Calling Conventions

**Current Z80 Convention**:
- Parameters in registers or SMC slots
- Return value in HL or A
- Shadow registers for fast context switch

**6502 Requirements**:
- Parameters in zero page or stack
- Return values in A or zero page
- Manual save/restore of registers

## 4. Code Generation Strategy

### 4.1 Proposed Architecture

```
MinZ Source → Parser → Semantic Analysis → MIR Generation
                                                 ↓
                                    Target-Independent Optimizer
                                                 ↓
                              Target Selection (Z80 | 6502)
                                      ↓              ↓
                              Z80 Backend    6502 Backend
                                      ↓              ↓
                              Z80 Assembly   6502 Assembly
```

### 4.2 Zero Page Allocation Strategy

For 6502, zero page would be organized as:
```
$00-$1F: Compiler temporaries (32 bytes)
$20-$7F: Local variables (96 bytes)
$80-$8F: Function parameters (16 bytes)
$90-$9F: System use (interrupts, etc.)
$A0-$FF: Available for user/system
```

### 4.3 Example Translation

**MinZ Source**:
```minz
fun add(x: u16, y: u16) -> u16 {
    return x + y
}
```

**Current Z80 Output**:
```asm
add:
    ; x in HL, y in DE
    ADD HL, DE
    RET
```

**Proposed 6502 Output**:
```asm
add:
    ; x at $80-81, y at $82-83, result at $84-85
    CLC
    LDA $80
    ADC $82
    STA $84
    LDA $81
    ADC $83
    STA $85
    RTS
```

## 5. Optimization Opportunities

### 5.1 6502-Specific Optimizations

1. **Zero Page Optimization**: Use zero page for frequently accessed variables
2. **Index Register Caching**: Keep array pointers in X/Y
3. **Accumulator Forwarding**: Chain operations through A
4. **Page Boundary Awareness**: Avoid page crossing penalties

### 5.2 SMC Adaptation for 6502

Instead of Z80-style immediate patching:
```asm
; 6502 TRUE SMC approach
smc_param:
    LDA #$00        ; Patch this immediate
    STA zero_page   ; Use as extended register
```

### 5.3 C64-Specific Features

1. **VIC-II Integration**: Direct screen memory access
2. **SID Programming**: Sound chip integration
3. **Sprite Handling**: Hardware sprite support
4. **Bank Switching**: Access to full 64KB despite ROM/IO

## 6. Effort Estimation

### 6.1 Major Tasks

1. **IR Abstraction** (3-4 weeks)
   - Abstract register model
   - Target-independent operations
   - Size-aware arithmetic

2. **6502 Backend** (6-8 weeks)
   - Register allocator for 3-register architecture
   - Zero page allocator
   - Instruction selection
   - 16-bit operation synthesis

3. **6502 Optimizations** (4-6 weeks)
   - Peephole optimizer
   - Zero page optimization
   - SMC adaptation

4. **C64 Runtime** (2-3 weeks)
   - Memory layout
   - I/O routines
   - Interrupt handlers

5. **Testing & Debugging** (3-4 weeks)
   - Test suite adaptation
   - C64 emulator integration
   - Performance benchmarking

**Total Estimate**: 18-25 weeks (4.5-6 months) for full implementation

### 6.2 Minimum Viable Implementation

A minimal 6502 backend without optimizations: 8-10 weeks
- Basic code generation
- Simple zero page allocation
- No SMC optimization
- Limited optimization passes

## 7. Strategic Considerations

### 7.1 Benefits

1. **Broader Market**: C64 has large retro computing community
2. **Architectural Diversity**: Proves MinZ's portability
3. **Technical Achievement**: Few languages target both Z80 and 6502 well
4. **Learning Opportunity**: 6502's constraints would improve IR design

### 7.2 Costs

1. **Development Time**: 4-6 months of focused effort
2. **Maintenance Burden**: Two backends to maintain
3. **Testing Complexity**: Double the test configurations
4. **Documentation**: Architecture-specific guides needed

### 7.3 Alternatives

1. **LLVM Backend**: Use LLVM IR instead of custom MIR
   - Pro: Multiple targets "for free"
   - Con: Loses SMC optimization, larger runtime

2. **Transpilation**: Generate C instead of assembly
   - Pro: Maximum portability
   - Con: Loses low-level control, efficiency

3. **Focus on Z80**: Perfect single-target support
   - Pro: Maximum optimization potential
   - Con: Limited market reach

## 8. Recommendation

### 8.1 Short Term (0-3 months)

**Focus on perfecting Z80 support**:
- Complete TRUE SMC implementation
- Optimize register allocation
- Enhance standard library
- Build community around ZX Spectrum development

### 8.2 Medium Term (3-6 months)

**Prepare for multi-target**:
- Refactor IR to be more target-agnostic
- Create abstraction layer for register allocation
- Design plugin architecture for backends

### 8.3 Long Term (6-12 months)

**Implement 6502 support** if:
- Strong community interest emerges
- Z80 implementation is stable
- Resources are available

### 8.4 Experimental Approach

Consider a **proof-of-concept** 6502 backend:
- Basic functionality only
- No optimizations initially
- Community-driven development
- Learn from implementation before full commitment

## 9. Technical Deep Dive: Key Challenges

### 9.1 Addressing Mode Mismatch

**Z80 Example**: `LD A, (HL)`
- Load A from memory pointed to by HL

**6502 Equivalent**: No direct equivalent!
Must use:
```asm
; Assuming HL equivalent in zero page $80-81
LDY #0
LDA ($80),Y
```

### 9.2 Interrupt Handling

**Z80**: Shadow registers enable 16 T-state interrupt handlers

**6502**: Must manually save/restore registers:
```asm
PHA         ; Save A
TXA
PHA         ; Save X
TYA
PHA         ; Save Y
; ... handler code
PLA
TAY         ; Restore Y
PLA
TAX         ; Restore X
PLA         ; Restore A
RTI
```

### 9.3 Multiplication/Division

**Z80**: Already requires routines (no native mul/div)

**6502**: Similar situation, but different optimal algorithms due to architecture

## 10. Conclusion

Compiling MIR to 6502 is **technically feasible** but requires significant engineering effort. The main challenges are:

1. **Register Scarcity**: 3 registers vs Z80's rich set
2. **16-bit Operations**: Must synthesize all 16-bit arithmetic
3. **SMC Adaptation**: Different strategies needed
4. **Addressing Modes**: Significant architectural differences

The effort would take an experienced developer 4-6 months for a production-quality implementation. A proof-of-concept could be done in 8-10 weeks.

**Recommendation**: Continue focusing on Z80 excellence while gradually refactoring the IR to be more target-agnostic. If community interest in 6502 support grows significantly, revisit this analysis and consider a phased implementation approach.

The MinZ language's design (static typing, explicit memory management, inline assembly) would work well on 6502, but the current implementation is deeply optimized for Z80. Success would require embracing 6502's unique characteristics rather than trying to emulate Z80 behavior.