# MinZ Multi-Target Architecture Implementation Guide

**Version:** 1.0  
**Date:** August 3, 2025  
**Prerequisites:** ARCHITECTURE.md  

---

## Quick Start Implementation

### Step 1: Create Core Interfaces (1 day)

```bash
# Create target interface foundation
mkdir -p minzc/pkg/targets
cd minzc/pkg/targets
```

**Create `interface.go`:**
```go
package targets

//go:generate go run golang.org/x/tools/cmd/stringer -type=FeatureFlag

type Target interface {
    Name() string
    Description() string
    Version() string
    
    GetCapabilities() TargetCapabilities
    CreateCodeGenerator(w io.Writer) CodeGenerator
    CreateOptimizer() Optimizer
    
    ValidateModule(module *ir.Module) error
    SupportsFeature(feature FeatureFlag) bool
}

type TargetCapabilities struct {
    WordSize           int
    RegisterCount      int
    HasSelfModifyingCode bool
    AssemblyFormat     string
    // ... (full definition from ARCHITECTURE.md)
}

type FeatureFlag uint32
const (
    FeatureLambdas FeatureFlag = 1 << iota
    FeatureInterfaces
    FeatureSMC
    // ... (full definition from ARCHITECTURE.md)
)
```

### Step 2: Create Target Registry (1 day)

**Create `registry.go`:**
```go
package targets

import (
    "fmt"
    "sync"
)

type TargetRegistry struct {
    targets map[string]Target
    mutex   sync.RWMutex
}

var globalRegistry = &TargetRegistry{
    targets: make(map[string]Target),
}

func RegisterTarget(target Target) error {
    globalRegistry.mutex.Lock()
    defer globalRegistry.mutex.Unlock()
    
    name := target.Name()
    if _, exists := globalRegistry.targets[name]; exists {
        return fmt.Errorf("target %s already registered", name)
    }
    
    globalRegistry.targets[name] = target
    return nil
}

func GetTarget(name string) (Target, error) {
    globalRegistry.mutex.RLock()
    defer globalRegistry.mutex.RUnlock()
    
    target, exists := globalRegistry.targets[name]
    if !exists {
        return nil, fmt.Errorf("unknown target: %s", name)
    }
    
    return target, nil
}

func ListTargets() []Target {
    globalRegistry.mutex.RLock()
    defer globalRegistry.mutex.RUnlock()
    
    var targets []Target
    for _, target := range globalRegistry.targets {
        targets = append(targets, target)
    }
    
    return targets
}

func GetDefaultTarget() Target {
    target, err := GetTarget("z80")
    if err != nil {
        panic("z80 target not registered")
    }
    return target
}
```

### Step 3: Migrate Z80 Backend (2 days)

**Create Z80 target implementation:**
```bash
mkdir -p minzc/pkg/targets/z80
cd minzc/pkg/targets/z80
```

**Create `z80.go`:**
```go
package z80

import (
    "io"
    "github.com/minz/minzc/pkg/targets"
    "github.com/minz/minzc/pkg/ir"
    "github.com/minz/minzc/pkg/codegen" // Import existing Z80 generator
)

type Z80Target struct{}

func init() {
    target := &Z80Target{}
    if err := targets.RegisterTarget(target); err != nil {
        panic(fmt.Sprintf("failed to register Z80 target: %v", err))
    }
}

func (z *Z80Target) Name() string {
    return "z80"
}

func (z *Z80Target) Description() string {
    return "Zilog Z80 8-bit microprocessor"
}

func (z *Z80Target) Version() string {
    return "1.0.0"
}

func (z *Z80Target) GetCapabilities() targets.TargetCapabilities {
    return targets.TargetCapabilities{
        WordSize:               8,
        RegisterCount:          7, // A, B, C, D, E, H, L
        HasShadowRegisters:     true,
        HasSelfModifyingCode:   true,
        HasConditionalJumps:    true,
        HasIndirectCalls:       true,
        AddressSpaceSize:       65536,
        CodeSegmentStart:       0x8000,
        DataSegmentStart:       0xF000,
        AssemblyFormat:         "sjasmplus",
        ExecutableFormat:       "a80",
        DefaultEntryPoint:      "main",
        SupportedOptimizations: []targets.OptimizationType{
            targets.OptimizationSMC,
            targets.OptimizationDJNZ,
            targets.OptimizationRegisterAllocation,
            targets.OptimizationPeephole,
        },
    }
}

func (z *Z80Target) CreateCodeGenerator(w io.Writer) targets.CodeGenerator {
    // Wrap existing Z80 generator to implement new interface
    return &Z80CodeGeneratorWrapper{
        generator: codegen.NewZ80Generator(w),
    }
}

func (z *Z80Target) CreateOptimizer() targets.Optimizer {
    return &Z80OptimizerWrapper{
        // Wrap existing optimizer
    }
}

func (z *Z80Target) ValidateModule(module *ir.Module) error {
    // Z80 supports all current MinZ features
    return nil
}

func (z *Z80Target) SupportsFeature(feature targets.FeatureFlag) bool {
    switch feature {
    case targets.FeatureLambdas,
         targets.FeatureInterfaces,
         targets.FeatureSMC,
         targets.FeatureIterators,
         targets.FeatureMetafunctions,
         targets.FeatureInlineAssembly:
        return true
    default:
        return false
    }
}

// Z80CodeGeneratorWrapper adapts existing Z80Generator to new interface
type Z80CodeGeneratorWrapper struct {
    generator *codegen.Z80Generator
}

func (w *Z80CodeGeneratorWrapper) Initialize(module *ir.Module) error {
    // Delegate to existing generator
    return w.generator.Generate(module)
}

// ... implement other CodeGenerator methods by delegating
```

### Step 4: Update CLI (1 day)

**Update `cmd/minzc/main.go`:**
```go
package main

import (
    "fmt"
    "os"
    "github.com/minz/minzc/pkg/targets"
    "github.com/minz/minzc/pkg/targets/z80" // Import to register Z80 target
    "github.com/spf13/cobra"
)

var (
    targetName string = "z80" // Default to Z80 for backward compatibility
    // ... existing flags
)

func init() {
    // New multi-target flags
    rootCmd.Flags().StringVar(&targetName, "target", "z80", "compilation target")
    rootCmd.Flags().BoolVar(&listTargets, "list-targets", false, "list available targets")
    
    // ... existing flags unchanged
}

func compile(sourceFile string) error {
    // Get target
    target, err := targets.GetTarget(targetName)
    if err != nil {
        return fmt.Errorf("target error: %w", err)
    }
    
    // ... rest of compilation pipeline using target
    
    // Generate code using target-specific generator
    generator := target.CreateCodeGenerator(outFile)
    return generator.Initialize(irModule)
}
```

### Step 5: Validation Testing (1 day)

**Create basic validation tests:**
```go
package targets_test

import (
    "testing"
    "github.com/minz/minzc/pkg/targets"
    _ "github.com/minz/minzc/pkg/targets/z80" // Register Z80 target
)

func TestTargetRegistration(t *testing.T) {
    // Test that Z80 target is registered
    target, err := targets.GetTarget("z80")
    if err != nil {
        t.Fatalf("Z80 target not registered: %v", err)
    }
    
    if target.Name() != "z80" {
        t.Errorf("Expected target name 'z80', got '%s'", target.Name())
    }
}

func TestBackwardCompatibility(t *testing.T) {
    // Test that all existing examples still compile
    examples := []string{
        "../../../examples/fibonacci.minz",
        "../../../examples/simple_add.minz",
        // ... add key examples
    }
    
    for _, example := range examples {
        t.Run(filepath.Base(example), func(t *testing.T) {
            err := compileExample(example, "z80")
            if err != nil {
                t.Errorf("Example %s failed: %v", example, err)
            }
        })
    }
}
```

---

## Phase-by-Phase Implementation

### Phase 1: Foundation (Week 1-2)

**Day 1-2: Core Interfaces**
- [ ] Create `pkg/targets/interface.go`
- [ ] Create `pkg/targets/registry.go`
- [ ] Add comprehensive documentation
- [ ] Create basic unit tests

**Day 3-4: Z80 Migration**
- [ ] Create `pkg/targets/z80/z80.go`
- [ ] Implement Z80Target struct
- [ ] Create wrapper classes for existing code
- [ ] Ensure all existing functionality works

**Day 5-7: CLI Integration**
- [ ] Update CLI with `--target` flag
- [ ] Add `--list-targets` command
- [ ] Add `--target-info` command
- [ ] Maintain backward compatibility

**Day 8-10: Validation**
- [ ] Run full test suite with new architecture
- [ ] Verify all examples compile unchanged
- [ ] Performance testing to ensure no regression
- [ ] Documentation updates

**Deliverables:**
- Working multi-target foundation
- Z80 target fully migrated
- 100% backward compatibility
- Comprehensive test coverage

### Phase 2: First Alternative Target (Week 3-6)

**Target Selection Decision Matrix:**

| Target | Toolchain Quality | Architecture Benefit | Development Effort | Strategic Value |
|--------|------------------|---------------------|-------------------|-----------------|
| **68000** | ⭐⭐⭐⭐ Good | ⭐⭐⭐⭐⭐ Excellent | ⭐⭐⭐ Medium | ⭐⭐⭐⭐ High |
| **WASM** | ⭐⭐⭐⭐⭐ Excellent | ⭐⭐⭐ Good | ⭐⭐⭐⭐ High | ⭐⭐⭐⭐⭐ Very High |
| **6502** | ⭐⭐ Poor | ⭐⭐⭐⭐ Very Good | ⭐⭐ Low | ⭐⭐⭐ Medium |

**Recommendation: Start with 68000**
- Mature Go assembler ecosystem
- Significant performance advantage over Z80
- Good validation of register allocation improvements
- Clear optimization mapping from Z80

**Week 3: 68000 Research & Setup**
- [ ] Research Go-based 68000 assemblers
- [ ] Set up development environment
- [ ] Create `pkg/targets/m68000/` structure
- [ ] Design 68000-specific optimizations

**Week 4: 68000 Code Generation**
- [ ] Implement M68000Target struct
- [ ] Create 68000 code generator
- [ ] Implement basic instruction mapping
- [ ] Test with simple examples

**Week 5: 68000 Optimizations**
- [ ] Implement register allocation for 68000
- [ ] Map Z80 optimizations to 68000 equivalents
- [ ] Implement DBRA optimization (superior to DJNZ)
- [ ] Performance benchmarking

**Week 6: Validation & Documentation**
- [ ] Cross-target testing framework
- [ ] Performance comparison reports
- [ ] Feature compatibility documentation
- [ ] User guide for 68000 target

### Phase 3: Third Target - WASM (Week 7-10)

**Week 7: WASM Research**
- [ ] Research Go WASM toolchains (Wazero, etc.)
- [ ] Design SMC workaround strategies
- [ ] Plan web-based development environment
- [ ] Create WASM target structure

**Week 8-9: WASM Implementation**
- [ ] Implement WASMTarget struct
- [ ] Create WASM code generator
- [ ] Implement SMC workarounds (lookup tables, specialization)
- [ ] Iterator optimization for WASM loops

**Week 10: Integration & Testing**
- [ ] Cross-target validation
- [ ] Web-based demo environment
- [ ] Performance analysis
- [ ] Documentation and examples

### Phase 4: Production Readiness (Week 11-12)

**Week 11: Polish & Optimization**
- [ ] Performance optimization across all targets
- [ ] Advanced cross-target features
- [ ] Comprehensive error handling
- [ ] Memory usage optimization

**Week 12: Release Preparation**
- [ ] Complete documentation
- [ ] Final testing and validation
- [ ] Performance benchmarking report
- [ ] Release packaging and distribution

---

## Critical Implementation Details

### 1. Maintaining Backward Compatibility

**Absolute Requirements:**
```go
// MUST WORK: All existing command lines
minzc program.minz -o program.a80 -O --enable-smc

// MUST WORK: All existing Go API calls
generator := codegen.NewZ80Generator(writer)
err := generator.Generate(module)

// MUST WORK: All existing test cases without modification
```

**Implementation Strategy:**
1. **Transparent Wrapping**: Existing APIs forward to new target system
2. **Default Behavior**: Z80 remains default, no behavior changes
3. **Gradual Migration**: Old APIs work alongside new ones
4. **Deprecation Warnings**: Guide users to new APIs over time

### 2. Performance Considerations

**Critical Performance Requirements:**
- Single-target compilation speed must not regress
- Z80 code quality must be identical or better
- Memory usage should scale linearly with target count
- Target selection overhead should be negligible

**Implementation Guidelines:**
```go
// Use lazy loading for target-specific resources
func (t *Target) LoadResources() error {
    if t.resources == nil {
        t.resources = loadTargetResources(t.Name())
    }
    return nil
}

// Cache expensive computations
var optimizationCache = make(map[string]*OptimizationResult)

// Use interfaces to minimize memory allocation
type CodeEmitter interface {
    Emit(format string, args ...interface{})
}
```

### 3. Error Handling Strategy

**Comprehensive Error Context:**
```go
type TargetError struct {
    Target   string
    Phase    string // "validation", "optimization", "codegen"
    Feature  string // Which feature caused the issue
    Line     int    // Source line if available
    Original error
}

func (te *TargetError) Error() string {
    return fmt.Sprintf("target %s (%s): %s at line %d: %v", 
        te.Target, te.Phase, te.Feature, te.Line, te.Original)
}

// Usage in target implementations
func (t *WASMTarget) ValidateModule(module *ir.Module) error {
    for _, fn := range module.Functions {
        if fn.HasInlineAssembly {
            return &TargetError{
                Target:   "wasm",
                Phase:    "validation", 
                Feature:  "inline_assembly",
                Line:     fn.SourceLine,
                Original: errors.New("WASM doesn't support inline assembly"),
            }
        }
    }
    return nil
}
```

### 4. Testing Strategy Implementation

**Multi-Target Test Runner:**
```go
// Cross-target test execution
func TestCrossTarget(t *testing.T) {
    testCases := []struct {
        name     string
        source   string
        targets  []string
        expected map[string]interface{}
    }{
        {
            name:   "Basic Function Call",
            source: `fun add(a: u8, b: u8) -> u8 { return a + b; }`,
            targets: []string{"z80", "68000", "wasm"},
            expected: map[string]interface{}{
                "z80":   CompileSuccess{},
                "68000": CompileSuccess{},
                "wasm":  CompileSuccess{},
            },
        },
    }
    
    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            for _, targetName := range tc.targets {
                t.Run(targetName, func(t *testing.T) {
                    result := compileForTarget(tc.source, targetName)
                    validateResult(t, result, tc.expected[targetName])
                })
            }
        })
    }
}
```

---

## Troubleshooting Guide

### Common Implementation Issues

**Issue 1: Target Registration Fails**
```
Error: target z80 already registered
```
**Solution:** Multiple imports causing duplicate registration
```go
// Fix: Use blank import in tests
import _ "github.com/minz/minzc/pkg/targets/z80"
```

**Issue 2: Existing Tests Fail**
```
Error: undefined: codegen.NewZ80Generator
```
**Solution:** Maintain backward compatibility wrapper
```go
// In pkg/codegen/z80.go (compatibility layer)
func NewZ80Generator(w io.Writer) *Z80Generator {
    target, _ := targets.GetTarget("z80")
    return target.CreateCodeGenerator(w).(*Z80Generator)
}
```

**Issue 3: Performance Regression**
```
Z80 compilation 20% slower after multi-target changes
```
**Solution:** Profile and optimize hot paths
```go
// Use benchmarking to identify bottlenecks
func BenchmarkZ80Compilation(b *testing.B) {
    for i := 0; i < b.N; i++ {
        compileForTarget(testSource, "z80")
    }
}
```

### Validation Checklist

**Before Each Phase:**
- [ ] All existing tests pass
- [ ] No performance regression on Z80
- [ ] Memory usage within acceptable limits
- [ ] Error messages are helpful and clear
- [ ] Documentation is updated

**Before Release:**
- [ ] Cross-target compatibility matrix complete
- [ ] Performance benchmarks documented
- [ ] All examples work on all supported targets
- [ ] CLI help is comprehensive and accurate
- [ ] API documentation is complete

---

## Success Metrics

### Phase 1 Success Criteria
- [ ] **100%** existing test pass rate maintained
- [ ] **0%** performance regression on Z80 target
- [ ] **Complete** backward compatibility for all CLI commands
- [ ] **Comprehensive** unit test coverage for new interfaces

### Phase 2 Success Criteria  
- [ ] **2 targets** fully functional (Z80 + one alternative)
- [ ] **Cross-target** test suite operational
- [ ] **Performance comparison** reports generated
- [ ] **Feature compatibility** matrix documented

### Final Success Criteria
- [ ] **3+ targets** production ready
- [ ] **Comprehensive** documentation for all targets
- [ ] **Performance optimization** demonstrated across targets
- [ ] **Community adoption** of multi-target features

---

## Next Steps

1. **Review Architecture**: Ensure ARCHITECTURE.md design meets requirements
2. **Set Up Development Environment**: Prepare workspace and tools
3. **Begin Phase 1**: Start with core interface implementation
4. **Continuous Validation**: Test at each step to maintain quality
5. **Community Engagement**: Share progress and gather feedback

This implementation guide provides the detailed roadmap for transforming MinZ into a true multi-target compiler while maintaining the quality and performance that defines the project.

---

**Document Status:** Complete  
**Implementation Ready:** Yes  
**Estimated Timeline:** 12 weeks for full multi-target support  
**Risk Level:** Low (with proper backward compatibility measures)