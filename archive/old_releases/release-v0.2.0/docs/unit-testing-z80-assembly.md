# Unit Testing Z80 Assembly: A Modern Approach for MinZ

## Introduction

Testing assembly code has traditionally been a painful process involving hardware debuggers, manual inspection, and crossed fingers. But what if we could bring modern unit testing practices to Z80 development? This article explores how to build a comprehensive testing framework for Z80 assembly using Go and the remogatto/z80 emulator.

## Why Unit Test Assembly Code?

### The Traditional Pain Points

1. **Debugging is Hard**: Finding bugs in assembly means stepping through CPU states, examining registers, and mentally tracking memory changes
2. **No Safety Net**: One wrong instruction can corrupt your entire program state
3. **Refactoring Fear**: Optimizing assembly code risks breaking working functionality
4. **Documentation**: Assembly code is notoriously hard to understand; tests serve as executable documentation

### The Benefits of Automated Testing

- **Rapid Development**: Test individual subroutines without loading to hardware
- **Regression Prevention**: Ensure optimizations don't break existing code
- **Documentation**: Tests clearly show what each routine does
- **Confidence**: Refactor and optimize without fear

## Architecture Overview

Our testing framework consists of three layers:

```
┌─────────────────────────────────┐
│      Test DSL (Given/When/Then) │  <- Readable test syntax
├─────────────────────────────────┤
│    MinZ Integration Layer       │  <- Symbol tables, calling conventions
├─────────────────────────────────┤
│    remogatto/z80 Emulator       │  <- Accurate Z80 emulation
└─────────────────────────────────┘
```

## Core Concepts

### 1. The Emulator Foundation

We use remogatto/z80 because it:
- Has proven accuracy (passes FUSE test suite)
- Provides clean interfaces for memory/IO hooks
- Is MIT licensed
- Offers good performance

### 2. Interface-Based Design

The key to testability is the emulator's interface design:

```go
type MemoryAccessor interface {
    ReadByte(address uint16) byte
    WriteByte(address uint16, value byte)
    // ... more methods
}
```

This allows us to inject test doubles that track all memory operations:

```go
type TestMemory struct {
    data   [65536]byte
    writes map[uint16][]byte  // Track all writes
    reads  map[uint16]int     // Count reads
}
```

### 3. DSL for Readability

Instead of cryptic setup code, we use a fluent interface:

```go
test.Given().
    Register("HL", 0x1234).
    Register("BC", 0x5678).
    Code(0x8000, 0x09, 0xC9)  // ADD HL,BC; RET

test.When().Execute(2)

test.Then().
    Register("HL", 0x68AC)
```

## Building the Test Framework

### Step 1: Basic Test Context

```go
type TestContext struct {
    cpu    *z80.Z80
    memory *TestMemory
    ports  *TestPorts
    t      *testing.T
}

func NewTest(t *testing.T) *TestContext {
    memory := NewTestMemory()
    ports := NewTestPorts()
    cpu := z80.NewZ80(memory, ports)
    return &TestContext{cpu, memory, ports, t}
}
```

### Step 2: Given/When/Then Pattern

The Given/When/Then pattern makes tests readable:

- **Given**: Set up initial state (registers, memory, code)
- **When**: Execute the code under test
- **Then**: Assert the expected outcomes

```go
func TestAddition(t *testing.T) {
    test := NewTest(t)
    
    test.Given().
        Register("A", 5).
        Register("B", 3).
        Code(0x8000, 0x80, 0xC9)  // ADD A,B; RET
    
    test.When().Execute(2)
    
    test.Then().
        Register("A", 8).
        Flag("Z", false).
        Flag("C", false)
}
```

### Step 3: Memory Protection Testing

One crucial aspect is testing memory protection:

```go
func TestROMProtection(t *testing.T) {
    test := NewTest(t)
    
    test.Given().
        Register("HL", 0x0000).  // ROM address
        Register("A", 0xFF).
        Memory(0x0000, 0x00).    // Initial ROM value
        Code(0x8000, 0x77, 0xC9) // LD (HL),A; RET
    
    test.When().Execute(2)
    
    test.Then().
        Memory(0x0000, 0x00)     // ROM unchanged!
}
```

### Step 4: MinZ Integration

For testing MinZ compiler output, we need to:

1. Load .a80 files (assembled output)
2. Parse symbol tables
3. Understand MinZ calling conventions

```go
type MinZTest struct {
    *TestContext
    symbols map[string]uint16
}

func (m *MinZTest) LoadA80(filename string) error {
    // Parse assembly output
    // Load code into memory
    // Extract labels and symbols
}

func (m *MinZTest) CallFunction(name string, args ...uint16) {
    addr := m.symbols[name]
    
    // MinZ calling convention:
    // - First arg in HL
    // - Second arg in DE  
    // - Additional args on stack
    // - Result in HL
    
    if len(args) > 0 {
        m.cpu.SetHL(args[0])
    }
    if len(args) > 1 {
        m.cpu.SetDE(args[1])
    }
    
    m.When().Call(addr)
}
```

## Real-World Examples

### Testing a Math Function

```go
func TestMinZMultiply(t *testing.T) {
    test := NewMinZTest(t)
    test.LoadA80("math.a80")
    test.LoadSymbols("math.sym")
    
    testCases := []struct {
        a, b, expected uint16
    }{
        {7, 8, 56},
        {255, 2, 510},
        {0, 100, 0},
    }
    
    for _, tc := range testCases {
        test.CallFunction("multiply", tc.a, tc.b)
        test.AssertResult(tc.expected)
    }
}
```

### Testing Array Operations

```go
func TestArraySum(t *testing.T) {
    test := NewMinZTest(t)
    test.LoadA80("arrays.a80")
    test.LoadSymbols("arrays.sym")
    
    // Set up test array
    arrayAddr := uint16(0x4000)
    test.Given().
        Memory(arrayAddr, 1, 2, 3, 4, 5)
    
    // Call sum_array(ptr, length)
    test.CallFunction("sum_array", arrayAddr, 5)
    
    test.AssertResult(15)  // 1+2+3+4+5
}
```

### Testing Interrupt Handlers

```go
func TestInterruptCounter(t *testing.T) {
    test := NewMinZTest(t)
    test.LoadA80("interrupts.a80")
    test.LoadSymbols("interrupts.sym")
    
    // Set up interrupt mode 2
    test.Given().
        Register("I", 0x80).
        Memory(0x8038, 0x00, 0x90)  // Vector table
    
    // Trigger 3 interrupts
    for i := 0; i < 3; i++ {
        test.When().Interrupt(false, 0x38)
    }
    
    // Check counter was incremented
    counterAddr := test.symbols["int_counter"]
    test.Then().
        Memory(counterAddr, 3, 0)
}
```

## Advanced Testing Patterns

### 1. Testing Self-Modifying Code

Self-modifying code (SMC) is common in Z80 optimization:

```go
func TestSMCOptimization(t *testing.T) {
    test := NewMinZTest(t)
    
    // First call - measures and modifies code
    test.CallFunction("adaptive_delay", 100)
    cycles1 := test.GetCycles()
    
    // Second call - uses optimized path
    test.CallFunction("adaptive_delay", 100)
    cycles2 := test.GetCycles()
    
    // Optimized version should be faster
    if cycles2 >= cycles1 {
        t.Error("SMC optimization failed")
    }
}
```

### 2. Testing Timing Constraints

For time-critical code:

```go
func TestVideoTiming(t *testing.T) {
    test := NewMinZTest(t)
    
    // Sprite routine must complete in one scanline
    test.CallFunction("draw_sprite", spriteAddr, x, y)
    
    test.Then().
        Cycles(0, 224)  // Max 224 T-states per scanline
}
```

### 3. Property-Based Testing

Test properties rather than specific values:

```go
func TestSortProperty(t *testing.T) {
    test := NewMinZTest(t)
    
    // Generate random array
    arr := generateRandomBytes(100)
    test.Given().Memory(0x4000, arr...)
    
    // Sort it
    test.CallFunction("quicksort", 0x4000, 100)
    
    // Verify sorted property
    sorted := test.ReadMemory(0x4000, 100)
    for i := 1; i < len(sorted); i++ {
        if sorted[i] < sorted[i-1] {
            t.Error("Array not sorted")
        }
    }
}
```

## Integration with CI/CD

The beauty of this approach is CI/CD integration:

```yaml
# .github/workflows/test.yml
name: Test MinZ Code
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Setup Go
        uses: actions/setup-go@v2
        
      - name: Build MinZ compiler
        run: cd minzc && make build
        
      - name: Compile test programs
        run: |
          minzc examples/math.minz -o test/math.a80
          minzc examples/arrays.minz -o test/arrays.a80
          
      - name: Run assembly tests
        run: go test ./test/...
```

## Performance Considerations

### Benchmark Support

The framework supports benchmarking:

```go
func BenchmarkMemcpy(b *testing.B) {
    test := NewMinZTest(&testing.T{})
    test.LoadA80("string.a80")
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        test.CallFunction("memcpy", 0x4000, 0x5000, 256)
    }
}
```

### Optimization Validation

Compare different implementations:

```go
func TestOptimizedVsNaive(t *testing.T) {
    test := NewMinZTest(t)
    
    // Test naive implementation
    start := test.GetCycles()
    test.CallFunction("multiply_naive", 50, 50)
    naiveCycles := test.GetCycles() - start
    result1 := test.GetResult()
    
    // Test optimized implementation
    start = test.GetCycles()
    test.CallFunction("multiply_optimized", 50, 50)
    optCycles := test.GetCycles() - start
    result2 := test.GetResult()
    
    // Same result, better performance
    if result1 != result2 {
        t.Error("Results differ")
    }
    if optCycles >= naiveCycles {
        t.Error("Optimization didn't improve performance")
    }
}
```

## Best Practices

### 1. Test at the Right Level

- **Unit tests**: Individual subroutines
- **Integration tests**: Module interactions
- **System tests**: Complete programs

### 2. Use Descriptive Names

```go
// Bad
func TestFunc1(t *testing.T)

// Good  
func TestMemcpy_CopiesCorrectly_WhenRangesOverlap(t *testing.T)
```

### 3. Test Edge Cases

```go
func TestDivision_EdgeCases(t *testing.T) {
    test := NewMinZTest(t)
    
    // Division by zero
    test.CallFunction("divide", 100, 0)
    test.Then().Flag("C", true)  // Carry = error
    
    // Maximum values
    test.CallFunction("divide", 0xFFFF, 1)
    test.AssertResult(0xFFFF)
}
```

### 4. Document Complex Tests

```go
// TestInterruptLatency verifies that our interrupt handler
// completes within the 32 T-state window between keyboard
// scans on the ZX Spectrum
func TestInterruptLatency(t *testing.T) {
    // ... test implementation
}
```

## Conclusion

Unit testing assembly code transforms Z80 development from a dark art into a systematic engineering practice. By combining modern testing patterns with accurate emulation, we can:

- Develop faster with immediate feedback
- Refactor confidently with regression tests  
- Document behavior through executable examples
- Optimize fearlessly with performance benchmarks

The framework presented here is just the beginning. As the MinZ ecosystem grows, we can extend it with:

- Coverage analysis
- Mutation testing
- Formal verification
- Visual debugging tools

Assembly programming doesn't have to be scary. With proper testing tools, it can be as systematic and reliable as any high-level language development.

## Resources

- [remogatto/z80 on GitHub](https://github.com/remogatto/z80)
- [MinZ Language Documentation](../README.md)
- [Z80 Instruction Set Reference](http://z80.info/z80code.htm)
- [Example test suite](../minzc/pkg/testing/)

Happy testing, and may your assembly code be bug-free!