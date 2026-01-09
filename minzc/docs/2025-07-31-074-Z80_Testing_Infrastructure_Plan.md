# Article 051: Z80 Testing Infrastructure Implementation Plan - SMC Tracing & Verification

**Author:** Claude Code Assistant  
**Date:** July 31, 2025  
**Version:** MinZ v0.6.0+  
**Status:** IMPLEMENTATION PLAN 🎯

## Executive Summary

This document outlines the immediate implementation plan for establishing comprehensive Z80 testing infrastructure with SMC (Self-Modifying Code) tracing capabilities. This is the **#1 priority** before any multi-backend work.

**Goal:** Verify MinZ compiler output executes correctly and TSMC provides claimed 30-40% performance improvements.

## 1. Current State Assessment

### 1.1 What We Have

✅ **Z80 Emulator Framework:**
```go
// pkg/testing/z80_test_framework.go
- remogatto/z80 emulator wrapper
- Memory/IO tracking
- Given/When/Then DSL
- Cycle counting
```

✅ **Partial Integration:**
```go
// pkg/testing/minz_integration.go
- A80 file loader (incomplete)
- Symbol table support (stub)
- MinZ-specific test helpers
```

### 1.2 What's Missing

❌ **Critical Gaps:**
1. No assembler integration (can't convert .a80 to binary)
2. No SMC tracing/verification
3. No end-to-end test execution
4. No performance benchmarking
5. No regression test suite

## 2. Implementation Phases (2 Weeks Total)

### Phase 1: Assembler Integration (Days 1-3)

#### 2.1.1 Install sjasmplus
```bash
# macOS
brew install sjasmplus

# Linux
apt-get install sjasmplus

# Or build from source
git clone https://github.com/z00m128/sjasmplus
cd sjasmplus
make
```

#### 2.1.2 Create Assembler Wrapper
```go
// pkg/testing/assembler.go
package testing

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
)

type Assembler interface {
    Assemble(sourceFile string) ([]byte, error)
    AssembleWithSymbols(sourceFile string) ([]byte, map[string]uint16, error)
}

type SjasmPlusAssembler struct {
    workDir string
}

func NewSjasmPlusAssembler() *SjasmPlusAssembler {
    workDir, _ := os.MkdirTemp("", "minz-asm-*")
    return &SjasmPlusAssembler{workDir: workDir}
}

func (a *SjasmPlusAssembler) Assemble(sourceFile string) ([]byte, error) {
    outFile := filepath.Join(a.workDir, "output.bin")
    
    cmd := exec.Command("sjasmplus", 
        sourceFile,
        "--output=" + outFile,
        "--nologo",
    )
    
    if err := cmd.Run(); err != nil {
        return nil, fmt.Errorf("assembly failed: %w", err)
    }
    
    return os.ReadFile(outFile)
}

func (a *SjasmPlusAssembler) AssembleWithSymbols(sourceFile string) ([]byte, map[string]uint16, error) {
    outFile := filepath.Join(a.workDir, "output.bin")
    symFile := filepath.Join(a.workDir, "output.sym")
    
    cmd := exec.Command("sjasmplus",
        sourceFile,
        "--output=" + outFile,
        "--sym=" + symFile,
        "--nologo",
    )
    
    if err := cmd.Run(); err != nil {
        return nil, nil, fmt.Errorf("assembly failed: %w", err)
    }
    
    binary, err := os.ReadFile(outFile)
    if err != nil {
        return nil, nil, err
    }
    
    symbols := parseSymbolFile(symFile)
    return binary, symbols, nil
}
```

### Phase 2: SMC Tracing Infrastructure (Days 4-6)

#### 2.2.1 SMC Memory Tracker
```go
// pkg/testing/smc_tracker.go
package testing

import (
    "fmt"
)

type SMCEvent struct {
    Cycle    int
    Address  uint16
    OldValue byte
    NewValue byte
    PC       uint16  // Where the write came from
}

type SMCTracker struct {
    events      []SMCEvent
    codeStart   uint16
    codeEnd     uint16
    enabled     bool
}

func NewSMCTracker(codeStart, codeEnd uint16) *SMCTracker {
    return &SMCTracker{
        codeStart: codeStart,
        codeEnd:   codeEnd,
        enabled:   true,
    }
}

func (t *SMCTracker) TrackWrite(address uint16, oldValue, newValue byte, pc uint16, cycle int) {
    if !t.enabled {
        return
    }
    
    // Check if write is to code region
    if address >= t.codeStart && address <= t.codeEnd {
        t.events = append(t.events, SMCEvent{
            Cycle:    cycle,
            Address:  address,
            OldValue: oldValue,
            NewValue: newValue,
            PC:       pc,
        })
    }
}

func (t *SMCTracker) GetEvents() []SMCEvent {
    return t.events
}

func (t *SMCTracker) Summary() string {
    if len(t.events) == 0 {
        return "No SMC events detected"
    }
    
    result := fmt.Sprintf("SMC Events: %d\n", len(t.events))
    for i, e := range t.events {
        result += fmt.Sprintf("[%d] Cycle %d: PC=%04X modified %04X from %02X to %02X\n",
            i, e.Cycle, e.PC, e.Address, e.OldValue, e.NewValue)
    }
    return result
}
```

#### 2.2.2 Enhanced Memory with SMC Tracking
```go
// pkg/testing/smc_memory.go
package testing

type SMCMemory struct {
    *TestMemory
    tracker *SMCTracker
    cpu     interface{ PC() uint16; Tstates() int }
}

func NewSMCMemory(codeStart, codeEnd uint16) *SMCMemory {
    return &SMCMemory{
        TestMemory: NewTestMemory(),
        tracker:    NewSMCTracker(codeStart, codeEnd),
    }
}

func (m *SMCMemory) SetCPU(cpu interface{ PC() uint16; Tstates() int }) {
    m.cpu = cpu
}

func (m *SMCMemory) WriteByte(address uint16, value byte) {
    oldValue := m.data[address]
    
    // Track before write
    if m.tracker != nil && m.cpu != nil {
        m.tracker.TrackWrite(address, oldValue, value, m.cpu.PC(), m.cpu.Tstates())
    }
    
    // Perform write
    m.TestMemory.WriteByte(address, value)
}
```

### Phase 3: End-to-End Test Harness (Days 7-9)

#### 2.3.1 Complete Test Pipeline
```go
// pkg/testing/e2e_harness.go
package testing

import (
    "path/filepath"
    "testing"
    "github.com/remogatto/z80"
)

type E2ETestHarness struct {
    compiler  *Compiler
    assembler Assembler
    emulator  *TestContext
    smcMemory *SMCMemory
}

func NewE2ETestHarness(t *testing.T) *E2ETestHarness {
    smcMem := NewSMCMemory(0x8000, 0xFFFF)
    ports := NewTestPorts()
    cpu := z80.NewZ80(smcMem, ports)
    smcMem.SetCPU(cpu)
    
    return &E2ETestHarness{
        compiler:  NewCompiler(),
        assembler: NewSjasmPlusAssembler(),
        emulator: &TestContext{
            cpu:    cpu,
            memory: smcMem.TestMemory,
            ports:  ports,
            t:      t,
        },
        smcMemory: smcMem,
    }
}

func (h *E2ETestHarness) CompileAndRun(source string) (*TestResult, error) {
    // Step 1: Compile MinZ to .a80
    asmFile, err := h.compiler.CompileToFile(source, "test.a80")
    if err != nil {
        return nil, fmt.Errorf("compilation failed: %w", err)
    }
    
    // Step 2: Assemble to binary
    binary, symbols, err := h.assembler.AssembleWithSymbols(asmFile)
    if err != nil {
        return nil, fmt.Errorf("assembly failed: %w", err)
    }
    
    // Step 3: Load into emulator
    h.emulator.memory.Load(0x8000, binary)
    
    // Step 4: Find and call main
    mainAddr, ok := symbols["main"]
    if !ok {
        return nil, fmt.Errorf("main function not found")
    }
    
    // Step 5: Execute
    startCycles := h.emulator.cpu.Tstates
    h.emulator.When().Call(mainAddr)
    endCycles := h.emulator.cpu.Tstates
    
    return &TestResult{
        Cycles:    endCycles - startCycles,
        SMCEvents: h.smcMemory.tracker.GetEvents(),
        Symbols:   symbols,
        Result:    h.emulator.cpu.HL(),
    }, nil
}

type TestResult struct {
    Cycles    int
    SMCEvents []SMCEvent
    Symbols   map[string]uint16
    Result    uint16
}
```

### Phase 4: TSMC Performance Verification (Days 10-12)

#### 2.4.1 TSMC Benchmark Suite
```go
// tests/e2e/tsmc_benchmarks_test.go
package e2e

import (
    "testing"
)

func TestTSMCPerformance(t *testing.T) {
    tests := []struct {
        name          string
        traditional   string
        tsmc          string
        expectedGain  float64  // Minimum expected improvement
    }{
        {
            name: "Simple Function Call",
            traditional: `
                fun add(a: u8, b: u8) -> u8 {
                    return a + b;
                }
                
                fun main() -> u8 {
                    let sum = 0;
                    for i in 0..100 {
                        sum += add(i, 1);
                    }
                    return sum;
                }
            `,
            tsmc: `
                @tsmc
                fun add(a: u8, b: u8) -> u8 {
                    return a + b;
                }
                
                fun main() -> u8 {
                    let sum = 0;
                    for i in 0..100 {
                        sum += add(i, 1);
                    }
                    return sum;
                }
            `,
            expectedGain: 0.30, // 30% improvement
        },
        {
            name: "Recursive Fibonacci",
            traditional: `
                fun fib(n: u8) -> u8 {
                    if n <= 1 { return n; }
                    return fib(n-1) + fib(n-2);
                }
                
                fun main() -> u8 {
                    return fib(10);
                }
            `,
            tsmc: `
                @tsmc
                fun fib(n: u8) -> u8 {
                    if n <= 1 { return n; }
                    return fib(n-1) + fib(n-2);
                }
                
                fun main() -> u8 {
                    return fib(10);
                }
            `,
            expectedGain: 0.35, // 35% improvement expected
        },
    }
    
    harness := NewE2ETestHarness(t)
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Run traditional version
            tradResult, err := harness.CompileAndRun(tt.traditional)
            if err != nil {
                t.Fatalf("Traditional version failed: %v", err)
            }
            
            // Run TSMC version  
            tsmcResult, err := harness.CompileAndRun(tt.tsmc)
            if err != nil {
                t.Fatalf("TSMC version failed: %v", err)
            }
            
            // Verify same result
            if tradResult.Result != tsmcResult.Result {
                t.Errorf("Results differ: traditional=%d, tsmc=%d", 
                    tradResult.Result, tsmcResult.Result)
            }
            
            // Verify SMC occurred
            if len(tsmcResult.SMCEvents) == 0 {
                t.Error("No SMC events detected in TSMC version")
            }
            
            // Calculate performance gain
            gain := float64(tradResult.Cycles - tsmcResult.Cycles) / float64(tradResult.Cycles)
            t.Logf("Performance gain: %.2f%% (traditional=%d, tsmc=%d cycles)",
                gain*100, tradResult.Cycles, tsmcResult.Cycles)
            
            // Verify minimum expected gain
            if gain < tt.expectedGain {
                t.Errorf("Performance gain %.2f%% below expected %.2f%%",
                    gain*100, tt.expectedGain*100)
            }
            
            // Log SMC events
            t.Logf("SMC Events:\n%s", tsmcResult.SMCEvents.Summary())
        })
    }
}
```

#### 2.4.2 SMC Pattern Verification
```go
// tests/e2e/smc_patterns_test.go
package e2e

func TestSMCPatterns(t *testing.T) {
    harness := NewE2ETestHarness(t)
    
    t.Run("Parameter Patching", func(t *testing.T) {
        source := `
            @tsmc
            fun multiply_by_constant(x: u8) -> u8 {
                return x * 5;  // This 5 should be patched
            }
            
            fun main() -> u8 {
                return multiply_by_constant(10);
            }
        `
        
        result, err := harness.CompileAndRun(source)
        if err != nil {
            t.Fatal(err)
        }
        
        // Verify SMC events
        events := result.SMCEvents
        if len(events) == 0 {
            t.Fatal("No SMC events detected")
        }
        
        // Should patch the immediate value 5
        found := false
        for _, e := range events {
            if e.OldValue == 5 {
                found = true
                t.Logf("Found parameter patch: %04X changed from %02X to %02X",
                    e.Address, e.OldValue, e.NewValue)
            }
        }
        
        if !found {
            t.Error("Expected to find patching of constant 5")
        }
    })
    
    t.Run("Recursive SMC", func(t *testing.T) {
        source := `
            @tsmc
            fun countdown(n: u8) -> u8 {
                if n == 0 { return 0; }
                return 1 + countdown(n - 1);
            }
            
            fun main() -> u8 {
                return countdown(5);
            }
        `
        
        result, err := harness.CompileAndRun(source)
        if err != nil {
            t.Fatal(err)
        }
        
        // Should see multiple SMC events for recursive calls
        if len(result.SMCEvents) < 5 {
            t.Errorf("Expected at least 5 SMC events, got %d", len(result.SMCEvents))
        }
    })
}
```

### Phase 5: Test Infrastructure Integration (Days 13-14)

#### 2.5.1 Makefile Updates
```makefile
# minzc/Makefile additions

# Test dependencies
test-deps:
	@which sjasmplus > /dev/null || (echo "Please install sjasmplus" && exit 1)
	go get github.com/remogatto/z80

# End-to-end tests
test-e2e: test-deps build
	go test ./tests/e2e/... -v

# Performance benchmarks
bench: test-deps build
	go test ./tests/e2e/... -bench=. -benchmem

# SMC verification
test-smc: test-deps build
	go test ./tests/e2e/... -run=SMC -v

# Full test suite
test-all: test test-e2e test-smc
```

#### 2.5.2 CI/CD Integration
```yaml
# .github/workflows/test.yml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Install sjasmplus
        run: |
          git clone https://github.com/z00m128/sjasmplus
          cd sjasmplus
          make
          sudo make install
      
      - name: Run tests
        run: |
          cd minzc
          make test-all
      
      - name: Run benchmarks
        run: |
          cd minzc
          make bench
      
      - name: Upload SMC traces
        uses: actions/upload-artifact@v2
        with:
          name: smc-traces
          path: minzc/tests/e2e/*.trace
```

## 3. Test Corpus Development

### 3.1 Core Test Programs
```
tests/corpus/
  basic/
    arithmetic.minz      # Basic math operations
    functions.minz       # Function calls
    loops.minz          # For/while loops
    arrays.minz         # Array operations
    structs.minz        # Struct usage
    
  optimization/
    tsmc_simple.minz    # Basic TSMC
    tsmc_recursive.minz # Recursive TSMC
    tsmc_loops.minz     # TSMC in loops
    register_spill.minz # Register pressure
    
  features/
    interfaces.minz     # Zero-cost interfaces
    pattern_match.minz  # Pattern matching
    modules.minz        # Module system
    abi_calls.minz      # @abi integration
    
  regression/
    issue_xxx.minz      # Specific bug fixes
```

### 3.2 Performance Baseline Suite
```minz
// tests/corpus/benchmarks/baseline.minz
// Establishes performance baseline for each optimization

@benchmark("add_traditional")
fun add_trad(a: u8, b: u8) -> u8 {
    return a + b;
}

@benchmark("add_tsmc") 
@tsmc
fun add_tsmc(a: u8, b: u8) -> u8 {
    return a + b;
}

// Test harness automatically compares cycle counts
```

## 4. SMC Analysis Tools

### 4.1 SMC Trace Visualizer
```go
// tools/smc_visualizer.go
package main

import (
    "fmt"
    "image"
    "image/color"
    "image/png"
)

func visualizeSMCTrace(events []SMCEvent, codeSize int) {
    // Create heatmap of SMC activity
    img := image.NewRGBA(image.Rect(0, 0, codeSize, 100))
    
    for _, event := range events {
        x := int(event.Address - 0x8000)
        y := event.Cycle * 100 / maxCycle
        
        // Color based on frequency
        img.Set(x, y, color.RGBA{255, 0, 0, 255})
    }
    
    // Save visualization
    f, _ := os.Create("smc_heatmap.png")
    png.Encode(f, img)
}
```

### 4.2 Performance Report Generator
```go
// tools/perf_report.go
func generatePerformanceReport(results []TestResult) {
    report := &PerformanceReport{
        Date: time.Now(),
    }
    
    for _, r := range results {
        report.AddTest(r.Name, r.Traditional, r.TSMC)
    }
    
    // Generate markdown report
    fmt.Printf("# MinZ Performance Report\n\n")
    fmt.Printf("## TSMC Performance Gains\n\n")
    fmt.Printf("| Test | Traditional | TSMC | Improvement |\n")
    fmt.Printf("|------|-------------|------|-------------|\n")
    
    for _, test := range report.Tests {
        gain := (test.TradCycles - test.TSMCCycles) * 100 / test.TradCycles
        fmt.Printf("| %s | %d | %d | %d%% |\n",
            test.Name, test.TradCycles, test.TSMCCycles, gain)
    }
}
```

## 5. Verification Checklist

### 5.1 Core Functionality
- [ ] Basic arithmetic operations execute correctly
- [ ] Function calls work with proper parameter passing
- [ ] Loops execute correct number of iterations
- [ ] Structs and arrays work correctly
- [ ] Module imports resolve properly

### 5.2 Optimization Verification
- [ ] TSMC functions modify their own code
- [ ] TSMC provides ≥30% performance improvement
- [ ] Register allocation minimizes spills
- [ ] Shadow registers used for interrupts
- [ ] Peephole optimizations apply correctly

### 5.3 Advanced Features
- [ ] Interfaces compile to zero-cost calls
- [ ] Pattern matching generates efficient code
- [ ] @abi integration works with assembly
- [ ] Inline assembly executes correctly
- [ ] Compile-time metaprogramming works

## 6. Success Metrics

### 6.1 Must Have (Week 1)
- ✅ All examples compile and execute
- ✅ TSMC shows measurable improvement
- ✅ No crashes or hangs
- ✅ Basic test suite passes

### 6.2 Should Have (Week 2)
- ✅ 30%+ performance gain verified
- ✅ SMC patterns documented
- ✅ Regression test suite
- ✅ CI/CD integration

### 6.3 Nice to Have (Future)
- 📊 Performance dashboard
- 🔍 SMC visualization tools
- 📈 Historical performance tracking
- 🎮 Real hardware testing

## 7. Implementation Priority

### Week 1: Core Infrastructure
1. **Day 1-2:** Assembler integration
2. **Day 3-4:** Basic e2e execution
3. **Day 5-6:** SMC tracking
4. **Day 7:** First working tests

### Week 2: Verification & Polish
1. **Day 8-9:** TSMC benchmarks
2. **Day 10-11:** Full test corpus
3. **Day 12-13:** CI/CD setup
4. **Day 14:** Documentation

## 8. Risk Mitigation

### 8.1 Technical Risks
- **sjasmplus compatibility** → Test multiple versions
- **Emulator accuracy** → Cross-check with other emulators
- **SMC detection** → Multiple tracking methods
- **Performance measurement** → Account for emulator overhead

### 8.2 Process Risks
- **Scope creep** → Focus on Z80 only
- **Perfect vs good** → Ship working version first
- **Tool dependencies** → Document all requirements

## 9. Next Steps After Testing

Once testing infrastructure is solid:

1. **Fix any bugs found** during testing
2. **Optimize based on** performance data
3. **Document TSMC patterns** that work best
4. **Only then consider** multi-backend refactoring

## 10. Conclusion

This plan provides a clear path to establishing robust Z80 testing with SMC tracing. The key is to start simple and build incrementally:

1. **Get basic execution working** (Days 1-3)
2. **Add SMC tracking** (Days 4-6)
3. **Build test suite** (Days 7-9)
4. **Verify performance** (Days 10-12)
5. **Polish and integrate** (Days 13-14)

With this infrastructure in place, we can finally verify that MinZ delivers on its promises before expanding to new architectures.

---

*Remember: No multi-backend work until we can prove the Z80 implementation works correctly and delivers the promised performance gains. Testing first, expansion second.*