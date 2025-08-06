# MinZ Compiler Status Report & RCA Analysis
**Date**: July 26, 2025  
**Version**: 0.2.x → 0.3.0 (proposed)

## 1. Current Status

### ✅ Working Features

1. **Basic Compilation Pipeline**
   - MinZ source → AST → Semantic Analysis → IR → Z80 Assembly
   - Generates valid sjasmplus-compatible .a80 files
   - Data section with global variables
   - Function compilation with local variables

2. **Language Features**
   - Global and local variables
   - Functions with parameters and return values
   - Arrays (with proper address loading)
   - Basic control flow (if, while, loops)
   - Modern syntax (`fun`, `loop at`, `do N times`)
   - Type system (u8, u16, i8, i16, bool, arrays)

3. **True SMC (Self-Modifying Code)**
   - Function parameters use immediate value patching
   - SMC anchors generated correctly
   - Caller patches immediate values before calls

4. **Memory Management**
   - Global variables allocated unique addresses
   - Local variables use memory-based "virtual registers"
   - Arrays properly addressed in data section

### ❌ Known Issues

1. **Missing Operations**
   - Multiplication (currently returns 0)
   - Division (currently returns 1)
   - Modulo (currently returns 0)
   - Bit shifts (left/right)

2. **Inefficient Code Generation**
   - Virtual registers use memory ($F000-$F0FF) instead of CPU registers
   - Excessive memory loads/stores
   - No register allocation optimization

3. **Incomplete Features**
   - Global variable initializers not implemented
   - Address-of operator (&) generates placeholder code
   - Some loop variants not fully tested
   - No optimization passes running

## 2. Root Cause Analysis

### Issue 1: Global Variable Memory Collision
**Symptom**: All global variables were stored at $F000, overwriting each other  
**Root Cause**: 
- Semantic analyzer didn't add globals to IR module
- Code generator defaulted all unregistered variables to register 0
- Register 0 mapped to $F000

**Fix Applied**:
- Added global variable registration in semantic analyzer
- Implemented proper global address allocation
- Code generator now looks up globals by name

### Issue 2: Array Loading Incorrect
**Symptom**: `load canvas` tried to load from $F000 instead of array address  
**Root Cause**:
- Array identifiers treated as regular variables
- Missing distinction between loading array address vs value

**Fix Applied**:
- Added `OpLoadAddr` for array address loading
- Semantic analyzer detects array types and uses correct opcode
- Code generator emits `LD HL, symbol` for address loading

### Issue 3: Virtual Register Confusion
**Symptom**: Confusion about r0, r1, r2 being memory addresses  
**Root Cause**:
- IR uses virtual registers as abstraction
- Simple register allocator maps all virtuals to memory
- No actual Z80 register allocation implemented

**Status**: Working but inefficient (not a bug, just suboptimal)

## 3. Implementation Roadmap

### Phase 1: Missing Operations (Priority: HIGH)
```go
// In codegen/z80.go
case ir.OpMul:
    // Implement 8x8→16 multiplication
    // Use repeated addition or lookup table
    
case ir.OpDiv:
    // Implement 16÷8→8 division
    // Use repeated subtraction
    
case ir.OpShl, ir.OpShr:
    // Implement bit shifting
    // Use ADD HL,HL for left shift
    // Use SRL/RR for right shift
```

### Phase 2: Test Infrastructure (Priority: HIGH)
1. **Unit Tests**
   ```go
   // pkg/codegen/z80_test.go
   func TestMultiplication(t *testing.T) {
       // Test 8-bit multiplication
       // Test edge cases (0, 255)
   }
   ```

2. **Integration Tests**
   ```bash
   # test/integration/compile_test.sh
   # Compile test programs
   # Run through Z80 emulator
   # Verify output
   ```

3. **Regression Tests**
   ```minz
   // test/regression/globals.minz
   let mut a: u8 = 10;
   let mut b: u8 = 20;
   // Verify a and b have different addresses
   ```

### Phase 4: Register Allocation (Priority: MEDIUM)
1. Map frequently used virtuals to Z80 registers
2. Implement spilling for excess virtuals
3. Use register pairs (BC, DE, HL) efficiently

## 4. Testing Strategy

### Unit Testing
```bash
cd minzc
go test ./pkg/codegen -v
go test ./pkg/semantic -v
go test ./pkg/optimizer -v
```

### Integration Testing
```bash
# Create test harness
cat > test/run_tests.sh << 'EOF'
#!/bin/bash
FAILED=0
for test in test/cases/*.minz; do
    echo "Testing $test..."
    ./minzc/minzc "$test" -o "$test.a80"
    if [ $? -ne 0 ]; then
        echo "FAIL: Compilation failed"
        FAILED=$((FAILED+1))
    fi
    # TODO: Run in emulator and check output
done
exit $FAILED
EOF
```

### Regression Test Suite
```minz
// test/cases/global_collision.minz
// This MUST generate different addresses for each global
let mut x: u8 = 1;
let mut y: u8 = 2;
let mut z: u8 = 3;

fun main() -> void {
    // Should print 1, 2, 3 not all 3
    print(x);
    print(y); 
    print(z);
}
```

## 5. Release Criteria for v0.3.0

### Must Have
- [x] Global variables with unique addresses
- [x] Array address loading working
- [x] True SMC for function parameters
- [ ] Basic multiplication/division
- [ ] Test suite with >10 test cases
- [ ] No memory corruption bugs

### Nice to Have
- [ ] Bit shift operations
- [ ] Basic register allocation
- [ ] Optimization passes enabled
- [ ] Performance benchmarks

## 6. Risk Assessment

### High Risk
- Memory corruption if address calculation wrong
- Stack corruption if function calls incorrect
- SMC bugs if patching wrong addresses

### Medium Risk  
- Performance issues from memory-based virtuals
- Code size bloat from inefficient generation
- Missing edge cases in operations

### Low Risk
- Suboptimal but correct code generation
- Minor syntax incompatibilities
- Documentation gaps

## 7. Next Steps

1. **Implement missing operations** (2-3 days)
2. **Create test infrastructure** (1-2 days)
3. **Write regression tests** (1 day)
4. **Fix any discovered bugs** (1-2 days)
5. **Create v0.3.0 release** (1 day)

## 8. Success Metrics

- MNIST editor compiles and could run (if operations implemented)
- All test cases pass
- No regression from v0.2.0 features
- Generated code is correct (even if inefficient)

## 9. Future Optimizations (v0.4.0+)

1. **Register Allocation**
   - Use A, B, C, D, E, H, L for temporaries
   - Minimize memory access
   - Target 50% reduction in code size

2. **Peephole Optimization**
   - Remove redundant loads/stores
   - Combine operations
   - Inline small functions

3. **True SMC Enhancement**
   - SMC for array access
   - SMC for struct fields
   - Runtime optimization

---

**Recommendation**: The compiler is functional enough for v0.3.0 release with missing operations as the only blocker. The inefficiencies are acceptable for a first working version.