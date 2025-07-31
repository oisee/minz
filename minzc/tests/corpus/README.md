# MinZ Test Corpus

This directory contains a comprehensive test corpus for the MinZ compiler, automatically generated from the examples directory. The corpus provides automated testing for all MinZ language features and ensures examples continue to work correctly.

## Structure

```
corpus/
├── README.md           # This file
├── manifest.json       # Test manifest with all test definitions
├── basic/             # Basic language feature tests
├── advanced/          # Advanced feature tests (structs, arrays, etc.)
├── optimization/      # Optimization tests (TSMC, tail recursion)
├── integration/       # Integration tests (@abi, inline assembly)
├── real_world/        # Real-world application tests
└── results/           # Test execution results
```

## Test Categories

### Basic (65 tests)
Basic language features including:
- Arithmetic operations
- Control flow (if/else, loops)
- Functions and parameters
- Variables and constants
- Simple type operations

### Advanced (16 tests)
Advanced language features:
- Structs and enums
- Arrays and array operations
- Bit fields and bit manipulation
- Memory operations
- String operations

### Optimization (18 tests)
Performance optimization features:
- True Self-Modifying Code (TSMC)
- Tail recursion optimization
- Register allocation tests
- Shadow register usage
- SMC optimization patterns

### Integration (17 tests)
External integration features:
- @abi attribute for assembly integration
- Inline assembly blocks
- Hardware port access
- Interrupt handlers
- Module imports

### Real World (17 tests)
Complete applications and utilities:
- Game engines and sprites
- Text editors
- MNIST handwriting recognition
- Database implementations (ZVDB)
- State machines

## Running Tests

### Run All Tests
```bash
./scripts/run_corpus_tests.sh
```

### Run with Verbose Output
```bash
./scripts/run_corpus_tests.sh -v
```

### Run Specific Category
```bash
./scripts/run_corpus_tests.sh -f basic
./scripts/run_corpus_tests.sh -f optimization
```

### Run Specific Test
```bash
./scripts/run_corpus_tests.sh -f fibonacci
```

### Set Custom Timeout
```bash
./scripts/run_corpus_tests.sh -t 120s
```

## Test Manifest Format

Each test in `manifest.json` has the following structure:

```json
{
  "name": "fibonacci",
  "source_file": "examples/fibonacci.minz",
  "category": "basic",
  "description": "Test for fibonacci",
  "expected_result": {
    "compile_success": true,
    "run_success": true,
    "exit_code": 0
  },
  "performance": {
    "max_cycles": 5000,
    "tsmc_improvement": 30.0
  },
  "tags": ["tsmc", "optimization"],
  "function_tests": [
    {
      "function_name": "fibonacci",
      "arguments": [10],
      "expected": 55,
      "max_cycles": 3000
    }
  ],
  "memory_checks": [
    {
      "address": 22528,
      "expected": [56]
    }
  ]
}
```

## Adding New Tests

### Automatic Generation
To regenerate the test corpus from examples:
```bash
python3 scripts/generate_corpus_tests.py
```

### Analyze Specific Example
To generate a test entry for a specific file:
```bash
python3 scripts/analyze_example.py examples/my_test.minz
```

### Manual Addition
You can manually edit `manifest.json` to add custom test entries with specific expectations.

## Test Features

### Compilation Testing
- Verifies files compile successfully
- Checks for expected compilation errors
- Validates generated code size

### Function Testing
- Calls specific functions with test arguments
- Verifies return values match expectations
- Measures cycle counts for performance

### TSMC Performance Testing
- Compares performance with and without TSMC
- Verifies optimization improvements
- Tracks SMC event counts

### Memory Verification
- Checks memory state after execution
- Validates screen buffer changes
- Verifies data structure layouts

### Port I/O Testing
- Tracks hardware port interactions
- Validates device communication
- Ensures correct I/O sequences

## Test Results

Test results are saved to `results/summary.json` after each run:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "duration": "45.3s",
  "total": 133,
  "passed": 125,
  "failed": 5,
  "skipped": 3
}
```

## Known Issues

Some tests may be skipped due to known issues:
- Tests marked with `BLOCKING` in known_issues are automatically skipped
- Incomplete language features may cause compilation failures
- Some examples may be work-in-progress

## Performance Expectations

TSMC-enabled tests should show:
- 30-70% cycle reduction for typical code
- 3-5x speedup for parameter-heavy functions
- Measurable SMC event counts

## Integration with CI

The corpus tests can be integrated into CI pipelines:

```yaml
- name: Run MinZ Corpus Tests
  run: |
    cd minzc
    ./scripts/run_corpus_tests.sh
```

## Debugging Failed Tests

To debug a specific failing test:

1. Run with verbose output: `./scripts/run_corpus_tests.sh -v -f test_name`
2. Check generated assembly: `examples/output/test_name.a80`
3. Examine compiler output in test logs
4. Use the Z80 emulator debugger if needed

## Maintenance

The test corpus should be updated when:
- New examples are added
- Language features are modified
- Performance characteristics change
- Bug fixes affect test behavior

Regular maintenance tasks:
- Run `generate_corpus_tests.py` after adding examples
- Update expected results when behavior changes
- Add regression tests for fixed bugs
- Monitor performance trends over time