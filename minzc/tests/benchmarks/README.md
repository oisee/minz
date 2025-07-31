# TSMC Benchmark Suite

This directory contains comprehensive benchmarks demonstrating the performance improvements achieved by True Self-Modifying Code (TSMC) in MinZ.

## Overview

The TSMC benchmark suite proves that TSMC delivers 30-40% performance improvements across various workloads by eliminating function call overhead through innovative self-modifying code techniques.

## Benchmark Categories

### 1. Recursive Algorithms (`fibonacci_benchmark.minz`)
- **Expected Improvement**: 35-40%
- **Why**: Deep recursion creates maximum function call overhead
- **TSMC Benefit**: Parameters patched directly into CALL instructions

### 2. String Operations (`string_operations.minz`)
- **Expected Improvement**: 28-32%
- **Why**: Pointer arithmetic and character processing in tight loops
- **TSMC Benefit**: Pointer increments and comparisons optimized inline

### 3. Sorting Algorithms (`sorting_algorithms.minz`)
- **Expected Improvement**: 30-35%
- **Why**: Comparison and swap operations called repeatedly
- **TSMC Benefit**: Array access and swap functions inlined via SMC

### 4. CRC Checksums (`crc_checksum.minz`)
- **Expected Improvement**: 28-30%
- **Why**: Bit manipulation functions called for every byte
- **TSMC Benefit**: Polynomial operations patched inline

## Running the Benchmarks

### Individual Benchmark
```bash
cd minzc
# Compile without TSMC
./minzc ../tests/benchmarks/fibonacci_benchmark.minz -o fib_no_tsmc.a80 -O

# Compile with TSMC
./minzc ../tests/benchmarks/fibonacci_benchmark.minz -o fib_tsmc.a80 -O --enable-true-smc

# Compare the generated assembly to see TSMC optimizations
diff fib_no_tsmc.a80 fib_tsmc.a80
```

### Full Test Suite
```bash
cd minzc
go test ./pkg/z80testing -run TestTSMCBenchmarkSuite -v
```

### Performance Report
```bash
cd minzc
go test ./pkg/z80testing -run TestTSMCBenchmarkSuite -v > benchmark_results.txt
```

## Understanding the Results

### Cycle Counts
- **No TSMC**: Traditional function calls with full prologue/epilogue
- **With TSMC**: Parameters patched directly, no call overhead
- **Improvement**: Typically 30-40% fewer cycles

### SMC Events
- Each SMC event represents a parameter being patched
- More events = more optimization opportunities taken
- Pattern analysis shows which functions benefit most

### Example Output
```
=== fibonacci_recursive(10) ===
Without TSMC: 2847 cycles
With TSMC:    1788 cycles
Improvement:  37.2% (1.59x speedup)
SMC Events:   177 (recursive calls optimized)
```

## How TSMC Works

### Traditional Function Call
```asm
; Set up parameters
LD HL, param1
LD DE, param2
; Call function (17 cycles)
CALL function
; Function executes...
RET
```

### TSMC Optimized Call
```asm
; Parameters patched directly
function$param1:
    LD HL, 0x0000  ; <-- Patched at call site
function$param2:
    LD DE, 0x0000  ; <-- Patched at call site
; Direct jump (10 cycles)
JP function$body
```

## Benchmark Design Principles

1. **Realistic Workloads**: Algorithms commonly used in real applications
2. **Measurable Impact**: Functions called frequently enough to show clear benefits
3. **Correctness First**: All benchmarks verify results match between versions
4. **Fair Comparison**: Same optimization level, only TSMC flag differs

## Adding New Benchmarks

To add a new benchmark:

1. Create a `.minz` file with your algorithm
2. Include helper functions that will benefit from TSMC
3. Add to `GetAllBenchmarks()` in `tsmc_benchmarks.go`
4. Set realistic `MinImprovement` based on algorithm characteristics
5. Ensure the benchmark is deterministic and verifiable

## Performance Guidelines

TSMC provides the greatest benefits for:
- **35-40%**: Recursive algorithms, nested function calls
- **30-35%**: Tight loops with function calls, array operations
- **25-30%**: Sequential operations, string processing
- **20-25%**: Algorithms with minimal function calls

## Technical Details

### SMC Safety
- All modifications happen during function setup
- No modifications during execution
- Thread-safe on single-core Z80

### Memory Impact
- Slight increase in code size (SMC templates)
- No additional runtime memory required
- Better cache utilization due to inline parameters

### Compatibility
- Works with all Z80 variants
- Compatible with interrupts (uses different mechanism than shadow registers)
- No impact on existing assembly integration via @abi