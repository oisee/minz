# MinZ Multi-Target Compiler Architecture

## Executive Summary

This document describes the comprehensive multi-target architecture for the MinZ compiler, enabling support for multiple CPU backends (Z80, 6502, 68000, x86, WASM, etc.) while maintaining the existing Z80-focused functionality. The design emphasizes clean separation of concerns, extensibility, and zero disruption to current Z80 workflows.

## 1. Current Architecture Analysis

### Existing Pipeline (Z80-Only)
```
MinZ Source → Tree-sitter Parser → AST → Semantic Analysis → MIR → Z80 Optimizer → Z80 CodeGen → .a80 Assembly
```

### Key Components
- **Frontend**: Tree-sitter grammar, AST, semantic analyzer
- **IR**: Machine-Independent Representation (MIR) 
- **Backend**: Z80-specific optimizer and code generator
- **Features**: SMC optimization, shadow registers, DJNZ loops, register allocation

## 2. Multi-Target Architecture Design

### 2.1 Core Principles

1. **Backward Compatibility**: Existing Z80 functionality remains unchanged
2. **Target Abstraction**: Clean interfaces separate target-specific code
3. **Shared Frontend**: Single parser and semantic analyzer for all targets
4. **Extensible Pipeline**: Easy addition of new targets without core changes
5. **Feature Matrix**: Graceful handling of target-specific capabilities

### 2.2 New Pipeline Architecture

```
MinZ Source
    ↓
Tree-sitter Parser → AST
    ↓
Semantic Analysis → Target-Agnostic MIR
    ↓
Target Selection & Validation
    ↓
Target-Specific MIR → Target Optimizer → Target CodeGen → Assembly/Object Code
```

## 3. Interface Design

### 3.1 Target Interface

```go
// Target represents a compilation target (CPU architecture)
type Target interface {
    // Identification
    Name() string
    Description() string
    Aliases() []string // e.g., ["z80", "zilog-z80"]
    
    // Capabilities
    GetCapabilities() TargetCapabilities
    SupportsFeature(feature FeatureFlag) bool
    
    // MIR Processing
    ValidateMIR(module *ir.Module) error
    TransformMIR(module *ir.Module) (*ir.Module, error)
    
    // Code Generation
    CreateOptimizer(config OptimizerConfig) Optimizer
    CreateCodeGenerator(config CodeGenConfig) CodeGenerator
    
    // Assembly Format
    GetAssemblyFormat() AssemblyFormat
    GetFileExtension() string // .a80, .s, .asm, etc.
}
```

### 3.2 Optimizer Interface

```go
// Optimizer performs target-specific optimizations
type Optimizer interface {
    // Standard optimization passes
    OptimizeModule(module *ir.Module, level OptimizationLevel) error
    
    // Target-specific passes
    GetAvailablePasses() []OptimizationPass
    EnablePass(pass OptimizationPass) error
    DisablePass(pass OptimizationPass) error
    
    // Analysis
    AnalyzeRegisterUsage(function *ir.Function) RegisterAnalysis
    DetectOptimizationOpportunities(module *ir.Module) []Opportunity
}
```

### 3.3 CodeGenerator Interface

```go
// CodeGenerator produces assembly code for a target
type CodeGenerator interface {
    // Core generation
    GenerateModule(module *ir.Module) ([]byte, error)
    GenerateFunction(function *ir.Function) ([]byte, error)
    
    // Assembly management
    SetOutputFormat(format AssemblyFormat)
    SetSymbolPrefix(prefix string)
    SetOrigin(address uint32)
    
    // Debugging support
    GenerateDebugInfo() DebugInfo
    GenerateSymbolTable() SymbolTable
}
```

### 3.4 Target Capabilities

```go
type TargetCapabilities struct {
    // Architecture properties
    WordSize         int    // 8, 16, 32, 64 bits
    AddressSize      int    // Address bus width
    Endianness       Endian // Big, Little, or Both
    
    // Register system
    RegisterCount    int
    RegisterSizes    []int  // Available register sizes
    HasIndexRegs     bool   // IX, IY style registers
    HasShadowRegs    bool   // Z80-style alternate registers
    
    // Memory model
    HasSegmentation  bool   // x86-style segmentation
    HasBanking       bool   // Memory banking/paging
    MaxMemorySize    uint64
    
    // Instruction features
    HasSelfModifyingCode bool   // SMC support
    HasConditionalBranches bool
    HasRelativeBranches  bool
    HasDirectJumps       bool  // Can jump to absolute addresses
    
    // Advanced features
    HasInterrupts    bool
    HasDMA           bool
    HasFloatingPoint bool
    HasVectorOps     bool
    
    // Optimization features
    HasLoopOptimizations bool // DJNZ-style instructions
    HasBitManipulation   bool
    HasMultiply          bool
    HasDivide            bool
}
```

## 4. Target Registry System

### 4.1 Dynamic Target Registration

```go
// TargetRegistry manages available compilation targets
type TargetRegistry struct {
    targets map[string]Target
    aliases map[string]string
}

func NewTargetRegistry() *TargetRegistry {
    registry := &TargetRegistry{
        targets: make(map[string]Target),
        aliases: make(map[string]string),
    }
    
    // Register built-in targets
    registry.RegisterTarget(NewZ80Target())
    registry.RegisterTarget(New6502Target())
    registry.RegisterTarget(New68000Target())
    
    return registry
}

func (r *TargetRegistry) RegisterTarget(target Target) error {
    name := target.Name()
    r.targets[name] = target
    
    // Register aliases
    for _, alias := range target.Aliases() {
        r.aliases[alias] = name
    }
    
    return nil
}

func (r *TargetRegistry) GetTarget(name string) (Target, error) {
    // Check direct name first
    if target, exists := r.targets[name]; exists {
        return target, nil
    }
    
    // Check aliases
    if canonical, exists := r.aliases[name]; exists {
        return r.targets[canonical], nil
    }
    
    return nil, fmt.Errorf("unknown target: %s", name)
}

func (r *TargetRegistry) ListTargets() []string {
    var names []string
    for name := range r.targets {
        names = append(names, name)
    }
    return names
}
```

### 4.2 Target Discovery

```go
// Automatic target discovery for external plugins
func (r *TargetRegistry) DiscoverTargets(pluginDir string) error {
    // Scan for .so/.dll files implementing Target interface
    // Load and register dynamically
    return nil
}
```

## 5. Feature Matrix & Compatibility

### 5.1 Feature Flags

```go
type FeatureFlag uint32

const (
    FeatureSMC FeatureFlag = 1 << iota
    FeatureShadowRegisters
    FeatureDJNZOptimization
    FeatureBitManipulation
    FeatureInlineAssembly
    FeatureInterrupts
    FeatureFloatingPoint
    FeatureLambdas
    FeatureInterfaces
    FeatureIterators
    FeatureMetafunctions
)
```

### 5.2 Target-Specific Feature Mapping

| Feature | Z80 | 6502 | 68000 | x86 | WASM | Handling |
|---------|-----|------|-------|-----|------|----------|
| SMC | ✅ | ✅ | ✅ | ❌ | ❌ | Fallback to normal calls |
| Shadow Regs | ✅ | ❌ | ❌ | ❌ | ❌ | Use regular registers |
| DJNZ | ✅ | DEC+BNE | DBRA | LOOP | ❌ | Target-specific loops |
| Bit Ops | ✅ | Partial | ✅ | ✅ | ✅ | Generate equivalent code |
| Lambdas | ✅ | ✅ | ✅ | ✅ | ✅ | Compile-time transform |
| Interfaces | ✅ | ✅ | ✅ | ✅ | ✅ | Zero-cost monomorphization |

### 5.3 Feature Compatibility Handling

```go
func (t *BaseTarget) HandleUnsupportedFeature(feature FeatureFlag, context *FeatureContext) error {
    switch feature {
    case FeatureSMC:
        // Fallback: Convert SMC to regular function calls
        return t.convertSMCToRegularCalls(context)
        
    case FeatureDJNZOptimization:
        // Fallback: Use target-specific loop constructs
        return t.generateTargetSpecificLoop(context)
        
    case FeatureShadowRegisters:
        // Fallback: Use regular register allocation
        return t.useRegularRegisters(context)
        
    default:
        return fmt.Errorf("feature %v not supported on target %s", feature, t.Name())
    }
}
```

## 6. CLI Design & User Interface

### 6.1 Enhanced CLI

```bash
# Current Z80 usage (unchanged)
mz program.minz -o program.a80

# Multi-target usage
mz --target=6502 program.minz -o program.s
mz --target=68000 program.minz -o program.asm
mz --target=wasm program.minz -o program.wasm

# List available targets
mz --list-targets

# Target information
mz --target-info=6502

# Feature compatibility check
mz --check-compatibility --target=wasm program.minz
```

### 6.2 Target-Specific Flags

```bash
# Z80-specific flags (existing)
mz --target=z80 --enable-smc --use-shadow-regs program.minz

# 6502-specific flags
mz --target=6502 --zero-page-vars --decimal-mode program.minz

# WASM-specific flags  
mz --target=wasm --optimize-size --export-table program.minz
```

### 6.3 Configuration Files

```yaml
# minz.yaml - Project configuration
project:
  name: "my-retro-game"
  version: "1.0.0"

targets:
  z80:
    enabled: true
    output: "build/z80/game.a80"
    features:
      smc: true
      shadow_registers: true
    optimization_level: 2
    
  6502:
    enabled: true
    output: "build/6502/game.s"
    features:
      zero_page_optimization: true
    optimization_level: 2
    
  68000:
    enabled: false
    output: "build/68k/game.asm"
```

## 7. Directory Structure

### 7.1 Reorganized Package Structure

```
minzc/
├── cmd/
│   ├── minzc/main.go              # Main CLI (enhanced)
│   └── repl/                      # REPL (target-aware)
├── pkg/
│   ├── frontend/                  # Shared frontend components
│   │   ├── parser/               # Tree-sitter integration
│   │   ├── ast/                  # Abstract syntax tree
│   │   └── semantic/             # Semantic analysis
│   ├── ir/                       # Machine-independent IR
│   │   ├── ir.go                 # Core IR definitions
│   │   ├── builder.go            # IR construction
│   │   └── transforms/           # Target-agnostic transforms
│   ├── targets/                  # Target implementations
│   │   ├── registry.go           # Target registry
│   │   ├── interfaces.go         # Target interfaces
│   │   ├── z80/                  # Z80 target (existing code)
│   │   │   ├── target.go
│   │   │   ├── optimizer.go
│   │   │   ├── codegen.go
│   │   │   └── registers.go
│   │   ├── m6502/                # 6502 target
│   │   │   ├── target.go
│   │   │   ├── optimizer.go
│   │   │   └── codegen.go
│   │   ├── m68000/               # 68000 target
│   │   └── wasm/                 # WebAssembly target
│   ├── optimizer/                # Shared optimization framework
│   │   ├── passes/               # Common optimization passes
│   │   └── analysis/             # Shared analysis tools
│   └── utils/                    # Shared utilities
└── targets/                      # External target plugins
    ├── custom-chip/
    └── fpga-soft-core/
```

### 7.2 Migration of Existing Code

```go
// Current Z80 code location → New location
minzc/pkg/codegen/z80.go          → minzc/pkg/targets/z80/codegen.go
minzc/pkg/optimizer/              → minzc/pkg/targets/z80/optimizer.go (Z80-specific)
                                   → minzc/pkg/optimizer/passes/ (generic passes)
```

## 8. Target Implementation Examples

### 8.1 Z80 Target (Existing, Refactored)

```go
type Z80Target struct {
    capabilities TargetCapabilities
}

func NewZ80Target() *Z80Target {
    return &Z80Target{
        capabilities: TargetCapabilities{
            WordSize:                8,
            AddressSize:            16,
            Endianness:             Little,
            RegisterCount:          8,
            RegisterSizes:          []int{8, 16},
            HasIndexRegs:           true,  // IX, IY
            HasShadowRegs:          true,  // Alternate registers
            HasSelfModifyingCode:   true,
            HasConditionalBranches: true,
            HasRelativeBranches:    true,
            HasDirectJumps:         true,
            HasInterrupts:          true,
            HasLoopOptimizations:   true,  // DJNZ
            HasBitManipulation:     true,
        },
    }
}

func (t *Z80Target) Name() string { return "z80" }
func (t *Z80Target) Aliases() []string { return []string{"zilog-z80", "z80a"} }

func (t *Z80Target) CreateOptimizer(config OptimizerConfig) Optimizer {
    return NewZ80Optimizer(config)
}

func (t *Z80Target) CreateCodeGenerator(config CodeGenConfig) CodeGenerator {
    return NewZ80CodeGenerator(config)
}
```

### 8.2 6502 Target (New)

```go
type M6502Target struct {
    capabilities TargetCapabilities
}

func NewM6502Target() *M6502Target {
    return &M6502Target{
        capabilities: TargetCapabilities{
            WordSize:               8,
            AddressSize:           16,
            Endianness:            Little,
            RegisterCount:         3,  // A, X, Y
            RegisterSizes:         []int{8, 16},
            HasIndexRegs:          true,  // X, Y
            HasShadowRegs:         false,
            HasSelfModifyingCode:  true,
            HasConditionalBranches: true,
            HasRelativeBranches:   true,
            HasDirectJumps:        true,
            HasInterrupts:         true,
            HasLoopOptimizations:  false, // No DJNZ equivalent
            HasBitManipulation:    false, // Limited bit ops
        },
    }
}

func (t *M6502Target) HandleUnsupportedFeature(feature FeatureFlag, context *FeatureContext) error {
    switch feature {
    case FeatureDJNZOptimization:
        // Convert DJNZ to DEC + BNE
        return t.generateDECBNELoop(context)
    case FeatureBitManipulation:
        // Generate bit manipulation sequences using AND/OR
        return t.generateBitSequences(context)
    default:
        return nil
    }
}
```

### 8.3 WebAssembly Target (New)

```go
type WASMTarget struct {
    capabilities TargetCapabilities
}

func NewWASMTarget() *WASMTarget {
    return &WASMTarget{
        capabilities: TargetCapabilities{
            WordSize:               32,
            AddressSize:           32,
            Endianness:            Little,
            RegisterCount:         0,  // Stack-based
            HasSelfModifyingCode:  false, // Not supported
            HasConditionalBranches: true,
            HasRelativeBranches:   false,
            HasDirectJumps:        true,
            HasInterrupts:         false,
            HasLoopOptimizations:  true,  // br_if loops
            HasBitManipulation:    true,
            HasFloatingPoint:      true,
        },
    }
}

func (t *WASMTarget) HandleUnsupportedFeature(feature FeatureFlag, context *FeatureContext) error {
    switch feature {
    case FeatureSMC:
        // Convert SMC to regular function calls with parameters
        return t.convertSMCToParameterizedCalls(context)
    case FeatureInterrupts:
        return fmt.Errorf("interrupts not supported in WebAssembly")
    default:
        return nil
    }
}
```

## 9. Migration Strategy

### 9.1 Phase 1: Interface Extraction (Week 1-2)

1. **Create target interfaces** without breaking existing code
2. **Extract Z80-specific code** into new target structure
3. **Maintain backward compatibility** with existing CLI
4. **Add target registry** with Z80 as default target

### 9.2 Phase 2: Pipeline Modification (Week 3-4)

1. **Enhance CLI** with target selection
2. **Modify compilation pipeline** to use target registry
3. **Add feature compatibility checking**
4. **Implement fallback mechanisms**

### 9.3 Phase 3: Target Implementation (Week 5-8)

1. **Implement 6502 target** as proof of concept
2. **Add comprehensive testing** for multi-target scenarios
3. **Create target-specific optimization passes**
4. **Document target development process**

### 9.4 Phase 4: Advanced Features (Week 9-12)

1. **Add WASM target** for modern deployment
2. **Implement 68000 target** for classic computers
3. **Create plugin system** for external targets
4. **Add cross-compilation support**

### 9.5 Breaking Changes Mitigation

```go
// Maintain backward compatibility with wrapper functions
func GenerateZ80(module *ir.Module, writer io.Writer) error {
    // Old API redirects to new multi-target system
    registry := GetDefaultRegistry()
    target, _ := registry.GetTarget("z80")
    codegen := target.CreateCodeGenerator(DefaultConfig())
    
    output, err := codegen.GenerateModule(module)
    if err != nil {
        return err
    }
    
    _, err = writer.Write(output)
    return err
}
```

## 10. Testing Strategy

### 10.1 Cross-Target Validation

```bash
# Test same program compiles for all targets
mz --target=z80 examples/fibonacci.minz -o fib.a80
mz --target=6502 examples/fibonacci.minz -o fib.s  
mz --target=68000 examples/fibonacci.minz -o fib.asm

# Verify output correctness through emulation/simulation
```

### 10.2 Feature Compatibility Testing

```go
func TestFeatureCompatibility(t *testing.T) {
    targets := []string{"z80", "6502", "68000", "wasm"}
    features := []string{"smc", "lambdas", "interfaces", "iterators"}
    
    for _, target := range targets {
        for _, feature := range features {
            t.Run(fmt.Sprintf("%s_%s", target, feature), func(t *testing.T) {
                testFeatureOnTarget(t, target, feature)
            })
        }
    }
}
```

### 10.3 Regression Testing

```bash
# Ensure Z80 functionality unchanged
./test_z80_regression.sh

# Test new targets don't break existing features
./test_multi_target_regression.sh
```

## 11. Performance Considerations

### 11.1 Compilation Speed

- **Lazy target loading**: Only load requested target
- **Shared IR optimization**: Common passes run once
- **Parallel compilation**: Generate multiple targets simultaneously

### 11.2 Output Quality

- **Target-specific optimizations**: Each target gets optimal code
- **Feature degradation**: Graceful fallbacks for unsupported features
- **Optimization level mapping**: O1/O2/O3 maps appropriately per target

## 12. Future Extensions

### 12.1 Additional Targets

- **8051**: Microcontroller applications
- **PIC**: Embedded systems
- **ARM Cortex-M**: Modern microcontrollers
- **RISC-V**: Open hardware platforms
- **Custom ASICs**: FPGA soft cores

### 12.2 Advanced Features

- **Cross-compilation**: Build on x86, target embedded
- **Retargeting**: Change target without recompiling frontend
- **Multi-target output**: Single source → multiple targets
- **Target-specific stdlib**: Optimized standard library per target

## 13. Documentation & Developer Experience

### 13.1 Target Development Guide

```markdown
# Creating a New MinZ Target

## 1. Implement Target Interface
## 2. Define Capabilities
## 3. Create Optimizer
## 4. Implement Code Generator
## 5. Add Tests
## 6. Register Target
```

### 13.2 Migration Guide

```markdown
# Migrating from Single-Target to Multi-Target

## For Users
- Add --target=z80 to maintain existing behavior
- Use minz.yaml for complex configurations

## For Developers
- Target interface replaces direct codegen calls
- Use target registry instead of hardcoded generators
```

## 14. Conclusion

This multi-target architecture provides MinZ with the flexibility to support multiple CPU architectures while preserving the existing Z80-focused development experience. The design emphasizes:

1. **Zero disruption** to existing Z80 workflows
2. **Clean abstractions** for easy target development  
3. **Graceful degradation** for unsupported features
4. **Extensible framework** for future targets
5. **Production-ready implementation** with proper testing

The phased implementation approach ensures that each step delivers value while building toward the complete multi-target vision. This architecture positions MinZ as a truly universal systems programming language for retro and embedded computing.

---

**Implementation Priority**: Start with Phase 1 (interface extraction) to establish the foundation, then proceed with 6502 target as the first alternate backend to validate the architecture design.