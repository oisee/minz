# MinZ TSMC Performance Testing Framework

## Overview

The TSMC Performance Testing Framework provides comprehensive benchmarking capabilities to measure and validate the performance improvements achieved by True Self-Modifying Code (TSMC) optimization in MinZ.

## Key Components

### 1. Benchmark Suite (`tsmc_benchmarks.go`)

The benchmark suite includes 10 diverse algorithms designed to stress different aspects of function calling:

- **Recursive Algorithms**: Deep call stacks (Fibonacci recursive)
- **Iterative Processing**: Loop-based algorithms with function calls
- **String Operations**: Character processing with validation
- **Array Operations**: Element access and manipulation
- **Mathematical Computations**: Factorial, prime checking, matrix multiplication
- **Sorting Algorithms**: Bubble sort with comparisons and swaps
- **Search Algorithms**: Binary search with tight loops
- **Real-world Algorithms**: CRC-8 checksum calculation

### 2. Performance Report Generator (`scripts/generate_performance_report.go`)

Automated report generation that produces:
- Markdown performance report with detailed analysis
- CSV data export for further analysis
- Performance dashboard with quick stats
- Visual charts and comparisons

### 3. Test Harness (`e2e_harness.go`)

End-to-end testing infrastructure that:
- Compiles MinZ code with/without TSMC
- Executes Z80 binaries in emulated environment
- Tracks cycle counts and SMC events
- Validates correctness of results

## Running Benchmarks

### Quick Start

```bash
# Run all TSMC benchmarks
make benchmark

# Generate performance report
make perf-report

# Run benchmarks and generate report
make benchmark-report
```

### Individual Benchmark Tests

```bash
# Run specific benchmark
go test -v ./pkg/z80testing -run TestTSMCBenchmark_Fibonacci

# Run with verbose output
go test -v ./pkg/z80testing -run TestTSMCBenchmarkSuite
```

### Custom Report Generation

```bash
# Generate report with custom options
go run scripts/generate_performance_report.go \
  -output ./reports \
  -csv=true \
  -markdown=true \
  -dashboard=true \
  -verbose=true
```

## Understanding Results

### Key Metrics

1. **Cycle Count**: Total Z80 T-states (clock cycles) for execution
2. **Improvement %**: Percentage reduction in cycles with TSMC
3. **Speedup Factor**: How many times faster (e.g., 1.5x = 50% faster)
4. **SMC Events**: Number of self-modifying code operations
5. **Unique Locations**: Distinct memory addresses modified

### Expected Performance

- **Target**: 30%+ average improvement across all benchmarks
- **Best Case**: 35-40% for recursive algorithms
- **Typical**: 28-35% for most workloads
- **Minimum**: 25% for simple iterative code

### SMC Patterns

The framework tracks several SMC patterns:

1. **Parameter Patching**: Direct modification of immediates
2. **Loop Counter Optimization**: In-place counter updates
3. **Jump Target Modification**: Dynamic branch optimization
4. **Register Allocation**: Runtime register assignment

## Adding New Benchmarks

To add a new benchmark:

1. Add to `GetAllBenchmarks()` in `tsmc_benchmarks.go`:

```go
{
    Name:        "your_benchmark",
    Description: "Brief description",
    Function:    "main_function_name",
    Args:        []uint16{arg1, arg2},
    MinImprovement: 30.0,  // Expected minimum %
    SourceCode: `
module your_module;

export fn your_function(param: u16) -> u16 {
    // Benchmark code
    return result;
}
`,
},
```

2. Ensure the benchmark:
   - Uses multiple function calls
   - Has measurable workload (>10K cycles)
   - Returns deterministic results
   - Tests a specific pattern

## Interpreting Reports

### Performance Report Sections

1. **Executive Summary**: High-level metrics and key findings
2. **Methodology**: Testing approach and measurement details
3. **Detailed Results**: Table with all benchmark data
4. **Category Analysis**: Performance grouped by algorithm type
5. **SMC Pattern Analysis**: Technical details of optimizations
6. **Real-World Impact**: Practical implications for development
7. **Developer Guidelines**: Best practices for using TSMC

### Performance Dashboard

Quick reference showing:
- Current performance stats
- Top performing benchmarks
- Category breakdown
- Platform compatibility status

## Troubleshooting

### Common Issues

1. **Benchmark Fails to Compile**
   - Check MinZ syntax in source code
   - Verify function names match
   - Ensure proper module structure

2. **Results Don't Match**
   - TSMC should produce identical results
   - Check for undefined behavior
   - Verify deterministic algorithms

3. **Low Performance Gains**
   - Ensure functions are being called
   - Check for already-optimized code
   - Verify TSMC is enabled

### Debug Options

```bash
# Run with debug output
MINZ_DEBUG=1 go test -v ./pkg/z80testing

# Generate detailed SMC trace
go test -v ./pkg/z80testing -run TestTSMCBenchmark_Fibonacci -smc-trace
```

## Future Enhancements

- Historical performance tracking
- Regression detection
- Platform-specific benchmarks
- Memory usage analysis
- Code size impact metrics
- Interactive benchmark explorer

## Contributing

When contributing benchmarks:
1. Follow existing patterns
2. Document algorithm purpose
3. Set realistic improvement targets
4. Include diverse workloads
5. Test on multiple inputs