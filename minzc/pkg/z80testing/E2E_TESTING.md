# MinZ End-to-End Testing Framework

This package provides comprehensive end-to-end testing for the MinZ compiler, with a focus on verifying TRUE SMC (Self-Modifying Code) performance improvements.

## Overview

The E2E test harness provides a complete pipeline for:
1. Compiling MinZ source code to .a80 assembly files
2. Assembling .a80 files to Z80 binary using sjasmplus
3. Loading binary into the remogatto/z80 emulator
4. Executing code with cycle-accurate tracking
5. Monitoring SMC events in real-time
6. Comparing TSMC vs non-TSMC performance

## Key Components

### E2ETestHarness

The main test harness that orchestrates the entire testing pipeline:

```go
h, err := NewE2ETestHarness(t)
defer h.Cleanup()

// Compile MinZ code
a80File, err := h.CompileMinZ("test.minz", true) // true = enable TSMC

// Assemble to binary
binary, symbols, err := h.AssembleA80(a80File)

// Load and execute
h.LoadBinary(binary, 0x8000)
err := h.CallFunction(symbols["myFunc"], arg1, arg2)

// Get results
result := h.GetResult()
cycles := h.GetCycles()
```

### Performance Comparison

Automated performance comparison between TSMC and non-TSMC versions:

```go
comparison, err := h.ComparePerformance("test.minz", "functionName", args...)

// Results include:
// - Cycle counts for both versions
// - Percentage improvement
// - Speedup factor
// - SMC event tracking
```

### SMC Tracking

Real-time tracking of self-modifying code events:

```go
// Get SMC statistics
stats := h.GetSMCStats()

// Get detailed SMC events
events := h.smcTracker.GetCodeEvents()

// Analyze SMC patterns
patterns := h.smcTracker.DetectPatterns()
```

## Running Tests

### Basic Test Run
```bash
cd /Users/alice/dev/minz-ts/minzc
go test -v ./pkg/z80testing -run TestE2E
```

### Run Specific Tests
```bash
# TSMC verification tests
go test -v ./pkg/z80testing -run TestTSMCPerformanceVerification

# Pattern analysis
go test -v ./pkg/z80testing -run TestTSMCPatternAnalysis

# Real-world benchmarks
go test -v ./pkg/z80testing -run TestTSMCRealWorldBenchmark
```

### Using the Test Runner Script
```bash
# Run all E2E tests
./scripts/run_e2e_tests.sh

# Run with benchmarks
./scripts/run_e2e_tests.sh --bench

# Generate coverage report
./scripts/run_e2e_tests.sh --coverage
```

## Test Files

Example MinZ test programs are provided in `test_files/`:

- `e2e_test_simple.minz` - Basic functionality tests
- `e2e_tsmc_demo.minz` - TSMC optimization demonstrations
- `tsmc_verification.minz` - Performance verification tests

## Writing E2E Tests

### Simple Test Example

```go
func TestMyFeature(t *testing.T) {
    h, err := NewE2ETestHarness(t)
    if err != nil {
        t.Fatalf("Failed to create harness: %v", err)
    }
    defer h.Cleanup()

    // Write MinZ source
    source := `
    fn add(a: u16, b: u16) -> u16 {
        return a + b;
    }
    `
    
    sourceFile := filepath.Join(h.workDir, "test.minz")
    ioutil.WriteFile(sourceFile, []byte(source), 0644)

    // Compare performance
    comparison, err := h.ComparePerformance(sourceFile, "add", 10, 20)
    if err != nil {
        t.Fatalf("Test failed: %v", err)
    }

    // Verify results
    if comparison.TSMCResult != 30 {
        t.Errorf("Wrong result: %d", comparison.TSMCResult)
    }

    // Check performance improvement
    comparison.AssertPerformanceImprovement(t, 25.0) // Expect 25%+ improvement
}
```

### Advanced Pattern Analysis

```go
// Analyze SMC patterns in TSMC code
funcAddr := symbols["myFunc"]
h.smcTracker.Clear()

err := h.CallFunction(funcAddr, args...)

// Get detailed SMC events
for _, event := range h.smcTracker.GetCodeEvents() {
    fmt.Printf("SMC: PC=%04X modified %04X: %02X -> %02X\n",
        event.PC, event.Address, event.OldValue, event.NewValue)
}
```

## Performance Targets

The TSMC optimization aims to achieve:
- **30%+ overall performance improvement** for typical code
- **40%+ improvement** for loop-heavy code
- **50%+ improvement** for parameter-intensive functions

## Troubleshooting

### Common Issues

1. **sjasmplus not found**
   - Ensure sjasmplus is installed at `/Users/alice/dev/bin/sjasmplus`
   - Or update the path in `e2e_harness.go`

2. **Compilation failures**
   - Check MinZ syntax in test files
   - Verify compiler flags for TSMC support

3. **No SMC events detected**
   - Ensure TSMC is enabled during compilation
   - Check that the code contains TSMC-optimizable patterns

### Debug Output

Enable detailed debugging:
```go
// In your test
t.Logf("SMC Summary:\n%s", h.GetSMCSummary())
```

## Future Enhancements

- Integration with code coverage tools
- Visual SMC event timeline
- Automated performance regression detection
- Support for multi-file MinZ projects
- Integration with CI/CD pipelines