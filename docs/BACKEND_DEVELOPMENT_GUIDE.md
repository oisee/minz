# MinZ Backend Development Guide

## Target Processor Specification

### 1. Compile-Time Target Selection
```bash
# Set default backend
minzc program.minz                    # Uses Z80 (default)
minzc program.minz -b 6502           # Target 6502
minzc program.minz -b wasm           # Target WebAssembly
```

### 2. @target Directive for Inline Assembly
```minz
// Default target (compile-time)
@asm {
    LD A, 42      ; Z80 assembly
}

// Explicit target selection
@target("z80") {
    @asm {
        EXX           ; Use shadow registers
        LD BC, 1000
    }
}

@target("6502") {
    @asm {
        LDA #42       ; 6502 assembly
        STA $00       ; Zero page!
    }
}

// Conditional compilation based on target
@if(TARGET == "6502") {
    // Use zero-page optimizations
    @asm { STA $00 }
} else if(TARGET == "z80") {
    // Use Z80 features
    @asm { LD (HL), A }
}
```

### 3. Backend-Specific Features
```minz
// Declare zero-page variables for 6502
@target("6502") {
    @zeropage global fast_counter: u8;   // Will use $00-$FF
    @zeropage global fast_ptr: u16;      // Two bytes in zero page
}

// Declare I/O ports for Z80
@target("z80") {
    @port(0xFE) border_color: u8;        // ZX Spectrum border
}
```

## Efficient Backend Development Framework

### 1. MIR-Based Code Generation Pipeline
```
MinZ AST → Semantic Analysis → MIR → Backend-Specific Transform → Target Code
                                 ↑
                          Common IR for all backends
```

### 2. Backend Implementation Structure
```go
type BackendImpl struct {
    // Configuration
    config BackendConfig
    
    // Code generation components
    regAlloc   RegisterAllocator
    instrSel   InstructionSelector
    optimizer  BackendOptimizer
    emitter    CodeEmitter
    
    // Target-specific features
    features   FeatureSet
}

// Minimal backend interface
type InstructionSelector interface {
    SelectInstructions(mir []MIRInstruction) []TargetInstruction
}

type RegisterAllocator interface {
    AllocateRegisters(instrs []TargetInstruction) error
}

type BackendOptimizer interface {
    Optimize(instrs []TargetInstruction) []TargetInstruction
}

type CodeEmitter interface {
    Emit(instrs []TargetInstruction) string
}
```

### 3. Shared Backend Utilities
```go
// Common utilities for all backends
package backendutil

// Register allocation graph coloring
func GraphColoringRegAlloc(cfg *ControlFlowGraph, numRegs int) *RegAllocation

// Basic block analysis
func BuildCFG(instructions []MIRInstruction) *ControlFlowGraph

// Peephole optimization patterns
func ApplyPeepholePatterns(instrs []Instruction, patterns []Pattern) []Instruction

// Instruction scheduling
func ScheduleInstructions(block *BasicBlock) []Instruction
```

## Zero-Page SMC Optimization for 6502

### Why Zero-Page is Perfect for SMC
1. **Fast Access**: Zero-page addressing is 1 cycle faster
2. **Compact Code**: 2-byte instructions instead of 3-byte
3. **Natural SMC**: Self-modifying zero-page code is common idiom

### Implementation Strategy
```minz
// MinZ code
fun draw_sprite(x: u8, y: u8, sprite: *u8) -> void {
    // Parameters will be SMC-patched in zero-page
}

// Generated 6502 code with zero-page SMC
draw_sprite:
    ; Parameters pre-patched in zero page
    LDA $F0       ; x coordinate (SMC location)
    STA screen_x
    LDA $F1       ; y coordinate (SMC location)
    STA screen_y
    LDA $F2       ; sprite pointer low (SMC location)
    STA sprite_lo
    LDA $F3       ; sprite pointer high (SMC location)
    STA sprite_hi
    ; ... rest of function
    RTS

; Calling code patches zero-page before JSR
call_draw_sprite:
    LDA #10
    STA $F0       ; Patch x parameter
    LDA #20  
    STA $F1       ; Patch y parameter
    LDA #<sprite_data
    STA $F2       ; Patch sprite pointer
    LDA #>sprite_data
    STA $F3
    JSR draw_sprite
```

### Zero-Page Allocation Strategy
```
$00-$0F: Temporary/scratch registers
$10-$1F: Function parameters (SMC)
$20-$3F: Fast global variables
$40-$7F: Reserved for user @zeropage
$80-$FF: System use (varies by platform)
```

## Backend Development Workflow

### 1. Start with MIR Interpreter
```go
// First implement a MIR interpreter for testing
type MIRInterpreter struct {
    registers map[int]Value
    memory    []byte
}

func (i *MIRInterpreter) Execute(mir []MIRInstruction) error {
    // Direct execution of MIR - helps understand semantics
}
```

### 2. Implement Basic Code Generation
```go
// Map each MIR instruction to target instructions
func (b *Backend) generateInstruction(mir MIRInstruction) []TargetInstr {
    switch mir.Op {
    case OpLoadConst:
        return []TargetInstr{
            {Op: "LDA", Operand: fmt.Sprintf("#$%02X", mir.Imm)},
        }
    // ... etc
    }
}
```

### 3. Add Register Allocation
- Start with simple linear scan
- Upgrade to graph coloring when needed
- Handle target-specific constraints

### 4. Implement Optimizations
- Peephole patterns first
- Instruction scheduling
- Target-specific optimizations (zero-page, etc.)

### 5. Testing Strategy
```bash
# Test harness for backends
minzc --test-backend 6502 test_suite/
minzc --compare-backends z80,6502 program.minz
minzc --benchmark-backend 6502 benchmarks/
```

## Example: Minimal 6502 Backend

```go
// Simplified 6502 backend showing key concepts
type M6502Backend struct {
    *BaseBackend
    zeroPageAlloc *ZeroPageAllocator
}

func (b *M6502Backend) Generate(mir *ir.Module) (string, error) {
    // 1. Allocate zero-page for SMC parameters
    b.allocateZeroPage(mir)
    
    // 2. Instruction selection
    targetInstrs := b.selectInstructions(mir)
    
    // 3. Register allocation (A, X, Y + zero-page as extended registers)
    b.allocateRegisters(targetInstrs)
    
    // 4. Apply 6502-specific optimizations
    targetInstrs = b.optimizeForZeroPage(targetInstrs)
    
    // 5. Emit assembly
    return b.emitAssembly(targetInstrs)
}

func (b *M6502Backend) optimizeForZeroPage(instrs []TargetInstr) []TargetInstr {
    // Convert absolute addressing to zero-page where possible
    // Implement zero-page SMC for function calls
    // Use zero-page for temporary values
}
```

## Backend Feature Matrix

| Feature | Z80 | 6502 | WASM | 68000 | ARM |
|---------|-----|------|------|-------|-----|
| Registers | 7×8-bit + pairs | 3×8-bit | ∞ locals | 8×32-bit | 16×32-bit |
| SMC Support | ✅ Full | ✅ Zero-page | ❌ | ✅ | ⚠️ Cache |
| Shadow Regs | ✅ | ❌ | N/A | ❌ | ❌ |
| Addressing | 16-bit | 16-bit + ZP | 32-bit | 32-bit | 32-bit |
| Calling Conv | Register | Stack/ZP | Stack | Register | Register |

## Next Steps

1. Implement @target directive parsing
2. Add zero-page SMC optimization for 6502
3. Create backend testing framework
4. Build shared optimization utilities
5. Document backend-specific features