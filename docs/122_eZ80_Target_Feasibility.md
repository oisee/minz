# 122. eZ80 Target Feasibility Analysis and Implementation Strategy

**Date**: 2025-08-02  
**Author**: Claude Code Research  
**Version**: 1.0  

## Executive Summary

This document analyzes the feasibility of adding eZ80 as a compilation target for the MinZ language compiler. The eZ80 processor, developed by Zilog as an enhanced Z80, provides compelling features including 24-bit addressing (16MB address space), native multiplication instructions, and 3-5x performance improvements while maintaining Z80 backward compatibility.

**Key Finding**: Adding eZ80 support is **highly feasible** and would provide significant value to MinZ users targeting modern embedded systems and calculator platforms (TI-84 Plus CE, etc.).

## 1. eZ80 Architecture Overview

### 1.1 Core Enhancements Over Z80

The eZ80 maintains Z80 binary compatibility while adding:

- **24-bit ALU** with extended arithmetic capabilities
- **16MB address space** (24-bit addressing in ADL mode)
- **Native multiplication** (MLT instruction for 8-bit × 8-bit = 16-bit)
- **3-stage pipeline** (fetch, decode, execute) providing 3-5x performance
- **Extended registers** (24-bit BC, DE, HL, IX, IY in ADL mode)
- **Dual memory modes** (Z80 compatibility mode and ADL mode)

### 1.2 Memory Modes

**Z80 Mode (Compatibility)**:
- 16-bit addressing with MBASE register extension
- 16-bit registers (BC, DE, HL, IX, IY)
- Full backward compatibility with existing Z80 code

**ADL Mode (Address Data Long)**:
- 24-bit linear addressing (16MB address space)
- 24-bit registers: {BCU,B,C}, {DEU,D,E}, {HLU,H,L}, etc.
- Enhanced instruction set with native multiplication
- 4x performance improvement over standard Z80

### 1.3 Register Architecture

```
Z80 Mode:          ADL Mode:
BC (16-bit)   →    {BCU, B, C} (24-bit)
DE (16-bit)   →    {DEU, D, E} (24-bit)  
HL (16-bit)   →    {HLU, H, L} (24-bit)
IX (16-bit)   →    {IXU, IXH, IXL} (24-bit)
IY (16-bit)   →    {IYU, IYH, IYL} (24-bit)
```

**Note**: Upper bytes (BCU, DEU, HLU, etc.) are not individually accessible as 8-bit registers.

## 2. Required MinZ Language Extensions

### 2.1 New Native Types

To fully leverage eZ80 capabilities, MinZ would need new 24-bit integer types:

```rust
// Current MinZ types (minzc/pkg/ir/ir.go:252-257)
TypeU8, TypeU16, TypeI8, TypeI16

// Proposed eZ80 extensions
TypeU24, TypeI24  // Native 24-bit integers for eZ80 ADL mode
```

**Code Changes Required**:
```go
// In minzc/pkg/ir/ir.go
const (
    TypeVoid TypeKind = iota
    TypeBool
    TypeU8
    TypeU16
    TypeI8
    TypeI16
    TypeU24  // NEW: 24-bit unsigned integer
    TypeI24  // NEW: 24-bit signed integer
)

func (t *BasicType) Size() int {
    switch t.Kind {
    case TypeVoid:
        return 0
    case TypeBool, TypeU8, TypeI8:
        return 1
    case TypeU16, TypeI16:
        return 2
    case TypeU24, TypeI24:  // NEW
        return 3
    default:
        return 0
    }
}
```

### 2.2 Enhanced Pointer Types

In ADL mode, pointers become 24-bit (3 bytes):

```go
// In minzc/pkg/ir/ir.go PointerType.Size()
func (t *PointerType) Size() int {
    if isEZ80ADLMode() {
        return 3  // 24-bit pointers in ADL mode
    }
    return 2      // 16-bit pointers in Z80 mode
}
```

### 2.3 Memory Model Changes

The 16MB address space enables new memory layouts:

```minz
// Current Z80 memory model (64KB)
let rom_start: *u8 = 0x0000 as *u8;
let ram_start: *u8 = 0x8000 as *u8;

// eZ80 ADL mode (16MB)
let rom_start: *u8 = 0x000000 as *u8;
let ram_start: *u8 = 0x800000 as *u8;
let extended_mem: *u8 = 0xD00000 as *u8;  // High memory
```

## 3. Implementation Strategy

### 3.1 Target Selection Architecture

**Proposed Command-Line Interface**:
```bash
# Current Z80 compilation (default)
minzc example.minz -o example.a80

# New eZ80 compilation options
minzc example.minz --target=ez80 -o example.a80        # Z80 compat mode
minzc example.minz --target=ez80-adl -o example.a80    # ADL mode
minzc example.minz --target=ez80-mixed -o example.a80  # Mixed mode
```

### 3.2 Multi-Backend Architecture

**Current Code Generator Structure**:
```go
// minzc/cmd/minzc/main.go:141
generator := codegen.NewZ80Generator(outFile)
```

**Proposed Enhanced Structure**:
```go
// New target-aware generator factory
func createGenerator(target string, outFile io.Writer) (CodeGenerator, error) {
    switch target {
    case "z80", "":
        return codegen.NewZ80Generator(outFile), nil
    case "ez80":
        return codegen.NewEZ80Generator(outFile, codegen.EZ80ModeCompat), nil
    case "ez80-adl":
        return codegen.NewEZ80Generator(outFile, codegen.EZ80ModeADL), nil
    case "ez80-mixed":
        return codegen.NewEZ80Generator(outFile, codegen.EZ80ModeMixed), nil
    default:
        return nil, fmt.Errorf("unsupported target: %s", target)
    }
}
```

### 3.3 Code Generator Implementation

**New file**: `minzc/pkg/codegen/ez80.go`

```go
package codegen

import (
    "io"
    "github.com/minz/minzc/pkg/ir"
)

type EZ80Mode int

const (
    EZ80ModeCompat EZ80Mode = iota  // Z80 compatibility mode
    EZ80ModeADL                     // 24-bit ADL mode
    EZ80ModeMixed                   // Mixed mode programming
)

type EZ80Generator struct {
    Z80Generator  // Embed existing Z80 generator
    mode         EZ80Mode
    adlEnabled   bool
}

func NewEZ80Generator(w io.Writer, mode EZ80Mode) *EZ80Generator {
    base := NewZ80Generator(w)
    return &EZ80Generator{
        Z80Generator: *base,
        mode:         mode,
        adlEnabled:   mode == EZ80ModeADL || mode == EZ80ModeMixed,
    }
}

func (g *EZ80Generator) Generate(module *ir.Module) error {
    g.writeEZ80Header()
    
    // Set appropriate mode
    switch g.mode {
    case EZ80ModeADL:
        g.emit(".ASSUME ADL=1")  // Enable ADL mode globally
    case EZ80ModeCompat:
        g.emit(".ASSUME ADL=0")  // Z80 compatibility mode
    case EZ80ModeMixed:
        g.emit(".ASSUME ADL=0")  // Start in Z80 mode, switch as needed
    }
    
    return g.Z80Generator.Generate(module)  // Delegate to base implementation
}
```

### 3.4 Register System Extensions

**Enhanced Register Definitions** (`minzc/pkg/ir/ir.go`):

```go
// Add eZ80-specific registers
const (
    // Existing Z80 registers...
    Z80_HL
    
    // New eZ80 extended registers
    EZ80_BCU    // Upper byte of BC in ADL mode
    EZ80_DEU    // Upper byte of DE in ADL mode  
    EZ80_HLU    // Upper byte of HL in ADL mode
    EZ80_IXU    // Upper byte of IX in ADL mode
    EZ80_IYU    // Upper byte of IY in ADL mode
)
```

## 4. eZ80-Specific Optimizations

### 4.1 Native Multiplication Optimization

The eZ80's MLT instruction provides significant performance benefits:

**Traditional Z80 Multiplication**:
```asm
; Multiply B × C (Z80 - requires subroutine call)
LD   A, B
LD   B, C
CALL MULTIPLY_8BIT    ; ~50-100+ T-states
```

**eZ80 Native Multiplication**:
```asm
; Multiply B × C (eZ80)
MLT  BC               ; 17 T-states (3-6x faster!)
```

**MinZ Compiler Integration**:
```go
// In optimizer package - new multiplication optimization pass
func (g *EZ80Generator) generateMul(dst, src1, src2 ir.Register) {
    if g.isEZ80Target() && g.canUseMLA(src1, src2) {
        // Use native MLT instruction
        g.emit("    LD   %s, %s", g.reg8(src1), g.operand(src2))
        g.emit("    LD   %s, %s", g.reg8pair(src1), g.operand(src1))
        g.emit("    MLT  %s", g.reg16pair(src1))
        return
    }
    // Fall back to Z80 multiplication routine
    g.generateZ80Mul(dst, src1, src2)
}
```

### 4.2 24-bit Address Optimization

**Large Memory Access Patterns**:
```minz
// MinZ code with large arrays
let large_buffer: [u8; 100000] = [0; 100000];  // 100KB array
large_buffer[50000] = 42;
```

**Generated eZ80 ADL Assembly**:
```asm
; Traditional Z80 - requires bank switching
LD   A, 3         ; Bank 3
OUT  (0xFE), A    ; Set memory bank
LD   HL, 0x2710   ; Offset within bank
LD   (HL), 42

; eZ80 ADL mode - direct access
LD   HL, 0x0C3500  ; Direct 24-bit address (50000)
LD   (HL), 42      ; Single instruction access
```

### 4.3 Performance-Critical Code Paths

**Loop Optimization with 24-bit Counters**:
```minz
// Large iteration count only possible on eZ80
for i in 0..1000000 {
    process_data(i);
}
```

**Generated eZ80 Code**:
```asm
; 24-bit loop counter (ADL mode)
    LD   HLU, 0x0F      ; Upper byte of 1000000
    LD   HL, 0x4240     ; Lower 16 bits
loop:
    CALL process_data
    DEC  HL.S           ; 24-bit decrement
    JR   NZ, loop       ; Continue if not zero
```

## 5. Backward Compatibility Strategy

### 5.1 Shared Code Generation

Most MinZ code can compile to both Z80 and eZ80:

```go
// Interface-based approach
type CodeGenerator interface {
    Generate(module *ir.Module) error
    GenerateFunction(fn *ir.Function) error
    GenerateInstruction(instr *ir.Instruction) error
}

// Z80Generator and EZ80Generator both implement CodeGenerator
// EZ80Generator embeds Z80Generator and overrides specific methods
```

### 5.2 Conditional Features

```minz
// MinZ code that adapts to target
#[cfg(target = "ez80")]
type ptr_size = u24;

#[cfg(target = "z80")]
type ptr_size = u16;

// Use native features when available
#[ez80_native]
fn fast_multiply(a: u8, b: u8) -> u16 {
    // Compiler generates MLT instruction for eZ80
    // Falls back to software multiply for Z80
    return a * b;
}
```

## 6. Performance Benefits Analysis

### 6.1 Expected Improvements

| Operation | Z80 T-States | eZ80 T-States | Improvement |
|-----------|--------------|---------------|-------------|
| 8×8 Multiply | 50-100+ | 17 | 3-6x faster |
| Memory Access | 7-13 | 6-9 | 1.3-2x faster |
| 24-bit Math | 30-50+ | 10-20 | 2-3x faster |
| Function Calls | 25-35 | 15-20 | 1.5-2x faster |

### 6.2 Real-World Use Cases

**Graphics Processing**:
```minz
// Pixel manipulation with large framebuffers
let vram: *u8 = 0xD40000 as *u8;  // 24-bit VRAM address
for y in 0..240 {
    for x in 0..320 {
        let pixel_addr = vram + (y * 320 + x);
        *pixel_addr = calculate_pixel(x, y);
    }
}
```

**Mathematical Computation**:
```minz
fn matrix_multiply_ez80(a: *[i16], b: *[i16], result: *[i16]) {
    // eZ80's MLT instruction dramatically speeds up
    // matrix multiplication operations
    for i in 0..size {
        for j in 0..size {
            let sum: i24 = 0;  // 24-bit accumulator
            for k in 0..size {
                sum += (a[i*size + k] * b[k*size + j]) as i24;
            }
            result[i*size + j] = sum as i16;
        }
    }
}
```

## 7. Implementation Roadmap

### Phase 1: Foundation (1-2 weeks)
- [ ] Add `--target` flag to minzc CLI
- [ ] Implement target selection in main.go
- [ ] Create basic EZ80Generator structure
- [ ] Add u24/i24 types to type system
- [ ] Update Size() methods for 24-bit types

### Phase 2: Code Generation (2-3 weeks)
- [ ] Implement EZ80Generator.Generate()
- [ ] Add ADL mode assembly directives
- [ ] Handle 24-bit pointer generation
- [ ] Implement mixed-mode support
- [ ] Add eZ80 register allocation

### Phase 3: Optimizations (2-3 weeks)
- [ ] MLT instruction optimization pass
- [ ] 24-bit arithmetic optimizations
- [ ] Large memory access patterns
- [ ] Performance-critical loop optimizations

### Phase 4: Testing & Integration (1-2 weeks)
- [ ] eZ80 test suite development
- [ ] Performance benchmarking
- [ ] TI-84 Plus CE target validation
- [ ] Documentation and examples

### Phase 5: Advanced Features (Optional)
- [ ] Mixed-mode programming support
- [ ] Advanced 24-bit TSMC optimizations
- [ ] eZ80-specific stdlib modules
- [ ] Hardware-specific optimizations

## 8. Technical Challenges & Solutions

### 8.1 Type System Integration

**Challenge**: Adding 24-bit types without breaking existing code.

**Solution**: Incremental type system extension with backward compatibility:
```go
// Gradual migration approach
func (t *BasicType) SizeForTarget(target Target) int {
    if target.IsEZ80() && target.IsADLMode() {
        // Handle 24-bit types for eZ80 ADL mode
        switch t.Kind {
        case TypeU24, TypeI24:
            return 3
        }
    }
    return t.Size()  // Existing Z80 behavior
}
```

### 8.2 Memory Model Complexity

**Challenge**: Supporting both 16-bit and 24-bit addressing in the same codebase.

**Solution**: Abstract memory model with target-specific implementations:
```go
type MemoryModel interface {
    PointerSize() int
    MaxAddress() uint32
    GenerateLoad(addr Address) string
    GenerateStore(addr Address, value string) string
}

type Z80MemoryModel struct{}
func (m Z80MemoryModel) PointerSize() int { return 2 }
func (m Z80MemoryModel) MaxAddress() uint32 { return 0xFFFF }

type EZ80ADLMemoryModel struct{}
func (m EZ80ADLMemoryModel) PointerSize() int { return 3 }
func (m EZ80ADLMemoryModel) MaxAddress() uint32 { return 0xFFFFFF }
```

### 8.3 Register Allocation Complexity

**Challenge**: eZ80's 24-bit registers in ADL mode vs 16-bit in Z80 mode.

**Solution**: Mode-aware register allocator:
```go
type EZ80RegisterAllocator struct {
    *Z80RegisterAllocator  // Embed base allocator
    adlMode              bool
    available24BitRegs   RegisterSet
}

func (a *EZ80RegisterAllocator) AllocateRegister(size int) (ir.Register, error) {
    if a.adlMode && size == 3 {
        // Allocate 24-bit register for ADL mode
        return a.allocate24BitRegister()
    }
    return a.Z80RegisterAllocator.AllocateRegister(size)
}
```

## 9. Community & Ecosystem Benefits

### 9.1 Target Platforms

**TI-84 Plus CE Calculator**:
- Popular in education (millions of units)
- eZ80 processor at 48MHz
- 3MB RAM, 4MB Flash
- Active homebrew development community

**Embedded Systems**:
- Modern Zilog eZ80 development boards
- Industrial control applications
- Retro computing projects with enhanced capabilities

### 9.2 Competitive Advantage

MinZ would be among the first modern systems languages to target eZ80:
- **C compilers** exist but lack modern language features
- **Assembly** is powerful but not productive
- **MinZ** provides modern syntax, safety, and Z80 heritage

## 10. Conclusion & Recommendation

### 10.1 Feasibility Assessment: **HIGHLY FEASIBLE**

✅ **Technical Feasibility**: The MinZ compiler architecture is well-designed for multi-target support  
✅ **Resource Requirements**: Estimated 6-8 weeks for full implementation  
✅ **Backward Compatibility**: Can maintain 100% Z80 compatibility  
✅ **Performance Benefits**: Significant improvements (2-6x) for mathematical code  
✅ **Market Demand**: Strong interest from TI calculator and embedded communities  

### 10.2 Strategic Value

1. **Differentiation**: First modern systems language targeting eZ80
2. **Performance**: Native multiplication and 24-bit arithmetic
3. **Address Space**: 16MB vs 64KB opens new application domains
4. **Future-Proofing**: Positions MinZ for next-generation Z80-based systems

### 10.3 Recommended Next Steps

1. **Spike Implementation** (1 week): Create basic `--target=ez80` support
2. **Community Validation** (1 week): Share with TI calculator community for feedback
3. **Full Implementation** (6-8 weeks): Complete eZ80 backend following roadmap
4. **Performance Benchmarking** (1 week): Validate claimed improvements
5. **Documentation** (1 week): Create eZ80-specific tutorials and examples

### 10.4 Implementation Priority

**HIGH PRIORITY** - This feature would significantly enhance MinZ's value proposition and market position. The eZ80's combination of Z80 compatibility with modern features aligns perfectly with MinZ's design philosophy of "modern language, retro targets."

The eZ80 backend would demonstrate MinZ's versatility and attract developers working on:
- Educational software (TI calculators)
- Modern embedded systems requiring more memory
- Performance-critical applications needing fast multiplication
- Projects bridging retro and modern computing

---

*This analysis demonstrates that eZ80 support is not only technically feasible but strategically valuable for the MinZ language ecosystem.*