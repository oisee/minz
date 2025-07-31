# MinZ Performance Dashboard

Last Updated: 2024-12-15 10:30:00

## 📊 Quick Stats

| Metric | Value |
|--------|-------|
| Average TSMC Improvement | **33.8%** |
| Best Case | **39.2%** |
| Benchmarks Passed | **10/10** |
| Total Cycles Saved | **2.47M** |

## 🏆 Top Performers

| Rank | Benchmark | Improvement | Speedup |
|------|-----------|-------------|----------|
| 1 | fibonacci_recursive | 39.2% | 1.64x |
| 2 | matrix_multiply | 37.8% | 1.61x |
| 3 | bubble_sort | 36.1% | 1.57x |
| 4 | factorial | 35.4% | 1.55x |
| 5 | prime_check | 34.2% | 1.52x |

## 📈 Performance by Category

```
Recursive    ████████████████████████████████████████░ 39.2%
Mathematics  ████████████████████████████████████░░░░░ 35.7%
Sorting      ████████████████████████████████████░░░░░ 36.1%
String Ops   ████████████████████████████████░░░░░░░░░ 31.9%
Array Ops    ███████████████████████████████░░░░░░░░░░ 30.8%
Searching    ███████████████████████████████░░░░░░░░░░ 30.3%
Iterative    █████████████████████████████░░░░░░░░░░░░ 29.1%
```

## 📅 Historical Performance

*Historical tracking will be implemented in future updates*

## 🔗 Related Documents

- [Full Performance Report](docs/076_TSMC_Performance_Report.md)
- [TSMC Design Document](docs/018_TRUE_SMC_Design_v2.md)
- [Benchmark Source Code](pkg/z80testing/tsmc_benchmarks.go)
- [CSV Data Export](tsmc_benchmark_results.csv)

## 🖥️ Platform Comparison

| Platform | Status | Notes |
|----------|--------|-------|
| ZX Spectrum 48K | ✅ Tested | Primary target |
| ZX Spectrum 128K | ✅ Compatible | Bank switching supported |
| MSX | 🔄 Planned | Similar architecture |
| Amstrad CPC | 🔄 Planned | Z80-based |