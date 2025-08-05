# 129. Backend Development Toolkit

## Overview

The MinZ Backend Development Toolkit provides a streamlined framework for creating new processor backends. It abstracts common patterns and provides reusable components, allowing new backends to be developed in hours rather than days.

## Quick Start

### Simple Backend in 20 Lines

```go
func NewSimpleBackend(options *BackendOptions) Backend {
    toolkit := NewBackendBuilder().
        WithInstruction(ir.OpLoadConst, "li %reg%, %value%").
        WithInstruction(ir.OpAdd, "add %dest%, %src1%, %src2%").
        WithPattern("load", "lw %reg%, %addr%").
        WithPattern("store", "sw %reg%, %addr%").
        WithCallConvention("registers", "a0").
        Build()
    
    return &SimpleBackend{toolkit: toolkit}
}
```

## Architecture

### Core Components

1. **BackendToolkit**: Configuration and patterns for code generation
2. **BaseGenerator**: Common code generation logic
3. **BackendBuilder**: Fluent API for configuration
4. **BackendPatterns**: Reusable code templates

### Key Concepts

#### Instruction Mappings
Direct 1:1 mappings from MIR opcodes to assembly:
```go
WithInstruction(ir.OpNop, "nop")
WithInstruction(ir.OpMove, "mov %dest%, %src%")
```

#### Pattern Templates
Complex operations using placeholders:
```go
WithPattern("add", "add %dest%, %src1%, %src2%")
WithPattern("load", "ld %reg%, [%addr%]")
```

#### Register Allocation
Map virtual MIR registers to physical registers:
```go
WithRegisterMapping(1, "a0")  // r1 -> a0
WithRegisterMapping(2, "a1")  // r2 -> a1
```

## Complete Example: 8-bit CPU Backend

```go
type My8BitBackend struct {
    options *BackendOptions
    toolkit *BackendToolkit
}

func NewMy8BitBackend(options *BackendOptions) Backend {
    toolkit := NewBackendBuilder().
        // Basic instructions
        WithInstruction(ir.OpNop, "nop").
        WithInstruction(ir.OpHalt, "halt").
        
        // Load/Store patterns
        WithPattern("load", "    ld a, %addr%").
        WithPattern("store", "    st %addr%, a").
        
        // Arithmetic (accumulator-based)
        WithPattern("add", "    add %src2%").
        WithPattern("sub", "    sub %src2%").
        
        // Function conventions
        WithPattern("prologue", "    push ix\n    ld ix, sp").
        WithPattern("epilogue", "    pop ix\n    ret").
        WithCallConvention("stack", "a").
        
        // Register mappings (limited on 8-bit)
        WithRegisterMapping(1, "a").    // Accumulator
        WithRegisterMapping(2, "b").    // B register
        WithRegisterMapping(3, "c").    // C register
        Build()
    
    return &My8BitBackend{
        options: options,
        toolkit: toolkit,
    }
}

func (b *My8BitBackend) Generate(module *ir.Module) (string, error) {
    gen := NewBaseGenerator(b, module, b.toolkit)
    return gen.Generate()
}
```

## Advanced Customization

### Custom Generator

For processors with unique requirements, extend BaseGenerator:

```go
type CustomGenerator struct {
    *BaseGenerator
    // Add custom state
    inInterrupt bool
}

func (g *CustomGenerator) GenerateFunction(fn *ir.Function) error {
    if fn.IsInterrupt {
        g.inInterrupt = true
        g.EmitLine("    push all")  // Save all registers
    }
    
    err := g.BaseGenerator.GenerateFunction(fn)
    
    if fn.IsInterrupt {
        g.EmitLine("    pop all")   // Restore all registers
        g.EmitLine("    reti")      // Return from interrupt
        g.inInterrupt = false
    }
    
    return err
}
```

### Platform-Specific Features

```go
func (b *MyBackend) SupportsFeature(feature string) bool {
    switch feature {
    case "memory_banking":
        return true  // Custom feature
    case "bit_addressable":
        return true  // Like 8051
    case FeatureSelfModifyingCode:
        return false // Harvard architecture
    default:
        return false
    }
}
```

## Pattern Reference

### Common Patterns

| Pattern | Purpose | Example |
|---------|---------|---------|
| `load` | Load from memory | `ld %reg%, %addr%` |
| `store` | Store to memory | `st %addr%, %reg%` |
| `add` | Addition | `add %dest%, %src1%, %src2%` |
| `sub` | Subtraction | `sub %dest%, %src1%, %src2%` |
| `mul` | Multiplication | `mul %dest%, %src1%, %src2%` |
| `div` | Division | `div %dest%, %src1%, %src2%` |
| `prologue` | Function entry | `push bp; mov bp, sp` |
| `epilogue` | Function exit | `pop bp; ret` |

### Placeholder Reference

| Placeholder | Replaced With | Example |
|-------------|---------------|---------|
| `%reg%` | Physical register | `r0`, `a`, `ax` |
| `%dest%` | Destination register | `r1` |
| `%src%`, `%src1%`, `%src2%` | Source registers | `r2`, `r3` |
| `%addr%` | Memory address | `data_var`, `0x1000` |
| `%value%` | Immediate value | `42`, `0xFF` |
| `%label%` | Jump label | `.L1`, `loop_start` |

## Best Practices

### 1. Start Simple
Begin with basic instructions and add complexity gradually:
```go
// Phase 1: Minimal viable backend
WithInstruction(ir.OpLoadConst, "li %reg%, %value%").
WithInstruction(ir.OpReturn, "ret")

// Phase 2: Add arithmetic
WithPattern("add", "add %dest%, %src1%, %src2%")

// Phase 3: Add optimizations
WithPattern("inc", "inc %reg%")  // Special case for +1
```

### 2. Reuse Base Functionality
Let BaseGenerator handle common tasks:
- Function headers/footers
- Label generation
- Name sanitization
- Debug comments

### 3. Override Strategically
Only override methods when necessary:
```go
// Good: Override for special handling
func (g *MyGenerator) GenerateCall(inst *ir.Instruction) error {
    if g.isBuiltin(inst.Symbol) {
        return g.generateBuiltinCall(inst)
    }
    return g.BaseGenerator.GenerateCall(inst)
}
```

### 4. Document Platform Specifics
```go
// Document unique features in comments
toolkit := NewBackendBuilder().
    // Z80: IX/IY are 16-bit index registers
    WithRegisterMapping(100, "ix").
    WithRegisterMapping(101, "iy").
    
    // Z80: DJNZ for efficient loops
    WithInstruction(ir.OpLoopDec, "djnz %label%").
    Build()
```

## Real-World Examples

### Z80 Backend (Excerpt)
```go
WithInstruction(ir.OpInc, "inc %reg%").
WithInstruction(ir.OpDec, "dec %reg%").
WithPattern("load", "    ld %reg%, %addr%").
WithPattern("store", "    ld %addr%, %reg%").
WithCallConvention("stack", "a")
```

### 6502 Backend (Excerpt)
```go
WithInstruction(ir.OpLoadConst, "lda #%value%").
WithPattern("load", "    lda %addr%").
WithPattern("store", "    sta %addr%").
WithCallConvention("zero-page", "a")
```

### 68000 Backend (Excerpt)
```go
WithInstruction(ir.OpMove, "move.%size% %src%, %dest%").
WithPattern("add", "    add.%size% %src2%, %dest%").
WithRegisterMapping(1, "d0").  // Data registers
WithRegisterMapping(9, "a0").  // Address registers
WithCallConvention("registers", "d0")
```

## Testing Your Backend

### 1. Unit Tests
```go
func TestMyBackend(t *testing.T) {
    backend := NewMyBackend(nil)
    module := &ir.Module{
        Functions: []*ir.Function{{
            Name: "test",
            Instructions: []ir.Instruction{
                {Op: ir.OpLoadConst, Dest: 1, Imm: 42},
                {Op: ir.OpReturn, Src1: 1},
            },
        }},
    }
    
    asm, err := backend.Generate(module)
    assert.NoError(t, err)
    assert.Contains(t, asm, "li a, 42")
}
```

### 2. Integration Tests
Test with real MinZ programs:
```bash
# Compile test program
./minzc -b mybackend test.minz -o test.s

# Verify output
grep "expected_instruction" test.s
```

### 3. Benchmark Tests
Compare with reference implementation:
```go
func BenchmarkMyBackend(b *testing.B) {
    backend := NewMyBackend(&BackendOptions{
        OptimizationLevel: 2,
    })
    // ... benchmark code generation
}
```

## Conclusion

The Backend Development Toolkit reduces the complexity of adding new processor support to MinZ. By providing common patterns and abstractions, it allows developers to focus on processor-specific details rather than boilerplate code.

With this toolkit, adding basic support for a new processor can be done in under 100 lines of code, while full-featured backends with optimizations remain manageable at 500-1000 lines.