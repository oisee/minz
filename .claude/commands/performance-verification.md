---
description: Verify performance claims with comprehensive benchmarking
allowed_tools: ["Task", "Write", "Bash", "Read"]
---

# Performance Verification Command

This command implements the performance verification pattern used to prove MinZ's TSMC optimization delivers 30%+ improvements.

## Methodology

1. **Baseline Establishment**
   - Compile code without optimizations
   - Measure cycle counts/execution time
   - Document memory usage

2. **Optimized Version**
   - Apply optimization techniques
   - Measure same metrics
   - Track optimization events (e.g., SMC modifications)

3. **Comparison Analysis**
   ```
   improvement = (baseline - optimized) / baseline * 100
   ```
   - Calculate percentage improvements
   - Verify correctness (same results)
   - Identify best/worst cases

4. **Report Generation**
   - Create markdown reports with tables
   - Generate ASCII charts for visualization
   - Export CSV for further analysis
   - Create performance dashboard

## Key Components:
- **Cycle-accurate measurement** - Use emulators/profilers
- **Multiple algorithms** - Test diverse workloads
- **Statistical validity** - Run multiple iterations
- **Visual presentation** - Charts and graphs

## Output:
- Performance report document
- Benchmark results data
- Visual dashboard
- Executive summary

$ARGUMENTS