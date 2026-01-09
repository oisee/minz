# MinZ v0.4.0 Roadmap: "Register Revolution"

**Target Release**: August 2025  
**Theme**: Physical register allocation and performance optimization

## 🎯 Major Goals

### 1. **Physical Z80 Register Allocation** (High Priority)
**Status**: Foundation exists, needs integration

**Current State**:
- ✅ Complete Z80RegisterAllocator framework implemented
- ✅ Shadow register support built-in
- ✅ Spill slot management designed
- ❌ Not integrated with code generation pipeline

**Implementation Plan**:
1. Integrate Z80RegisterAllocator into `generateFunction()`
2. Replace virtual register memory addresses with physical registers
3. Add register spilling for complex functions
4. Implement register coalescing for better performance

**Expected Benefits**:
- 60-80% performance improvement (registers vs memory)
- Smaller code size (fewer LD instructions)
- Better Z80 instruction utilization (register-specific ops)

### 2. **Stack-Based Local Variables** (Medium Priority)
**Current State**: All locals use global memory addresses ($F002, $F006, etc.)

**Target**: IX-based stack frame with offset addressing
```asm
; Current (inefficient):
LD HL, ($F002)  ; local variable 'a'
LD HL, ($F006)  ; local variable 'b'

; Target (efficient):
LD H, (IX+4)    ; local variable 'a' 
LD L, (IX+6)    ; local variable 'b'
```

**Implementation**:
1. Add stack frame setup in function prologue
2. Calculate local variable offsets from IX
3. Update OpLoadVar/OpStoreVar for IX+offset addressing
4. Implement proper stack cleanup in epilogue

### 3. **Function Parameter Optimization** (Medium Priority)
**Current**: Parameters passed via SMC (self-modifying code)
**Enhancement**: Optimize parameter passing strategies

**Options to Implement**:
- Register parameter passing for simple functions
- Stack parameter passing for complex functions  
- Hybrid approach based on function analysis
- Better SMC parameter slot reuse

## 🚀 Secondary Features

### 4. **Signed Arithmetic Operations**
**Status**: i8/i16 types exist but use unsigned operations

**Missing Operations**:
- Signed multiplication (different from unsigned)
- Signed division with proper sign handling
- Signed comparison operations (>, <, >=, <=)
- Sign extension for mixed operations

### 5. **Advanced Optimization Passes**
- **Dead Code Elimination**: Remove unused variables and operations
- **Constant Propagation**: Better compile-time evaluation
- **Loop Optimization**: Optimize common loop patterns
- **Peephole Optimization**: Pattern-based instruction improvements

### 6. **Enhanced Type System**
- Better type inference for complex expressions
- Implicit type conversions where safe
- Type-based operation selection improvements
- Warning system for potential type issues

## 📊 Performance Targets

### Code Size Reduction
- **Current**: ~28 instructions for simple function
- **Target**: ~13-15 instructions (54% reduction achieved in SMC tests)

### Execution Speed
- **Current**: ~400 T-states for basic operations  
- **Target**: ~150-200 T-states (62% improvement)

### Memory Usage
- **Current**: Each local uses 2-4 bytes of global memory
- **Target**: Locals on stack, only actual data size used

## 🛠️ Implementation Strategy

### Phase 1: Physical Register Integration (Week 1-2)
1. **Modify Code Generator**:
   - Integrate Z80RegisterAllocator into `generateFunction()`
   - Update all IR instruction handlers to use physical registers
   - Add register conflict resolution

2. **Testing**:
   - Test with simple functions first
   - Gradually add complexity (loops, conditionals)
   - Verify correctness with existing test suite

### Phase 2: Stack Frame Implementation (Week 3)
1. **Stack Management**:
   - Add IX-based stack frame setup
   - Calculate optimal local variable layout
   - Update load/store operations

2. **Integration**:
   - Modify RegisterAllocator to track stack offsets
   - Update OpLoadVar/OpStoreVar implementations
   - Add proper prologue/epilogue generation

### Phase 3: Optimization and Polish (Week 4)
1. **Advanced Features**:
   - Implement signed arithmetic
   - Add optimization passes
   - Performance tuning

2. **Documentation and Testing**:
   - Update compiler architecture docs
   - Create performance benchmarks
   - Comprehensive test suite

## 🧪 Testing Strategy

### Regression Testing
- All v0.3.2 features must continue working
- Existing examples should produce correct output
- Performance should improve, not regress

### New Feature Testing
- Create test cases for physical register allocation
- Stack frame tests with nested function calls
- Signed arithmetic operation validation
- Memory usage verification

### Performance Benchmarks
- Before/after comparison for key examples
- T-state counting for critical operations
- Memory usage analysis
- Code size measurements

## 📈 Success Metrics

### Functionality
- ✅ All existing examples compile and run correctly
- ✅ Physical registers used instead of memory for temporaries
- ✅ Stack-based locals with IX+offset addressing
- ✅ Signed operations work correctly

### Performance
- 🎯 50%+ reduction in T-states for arithmetic operations
- 🎯 30%+ reduction in generated code size
- 🎯 70%+ reduction in memory usage for locals

### Code Quality
- Clean register allocation with minimal spilling
- Optimal Z80 instruction selection
- Proper register lifetime management
- Efficient stack frame layout

## 🔮 Post-v0.4.0 Vision

### v0.5.0 Candidates
- Array element assignment and iteration
- User-defined modules and packages
- Inline assembly improvements
- Debugger integration

### Long-term Goals
- Full optimization pipeline
- Advanced language features
- IDE integration improvements
- Package management system

## 💡 Risk Assessment

### High Risk
- Register allocation complexity could introduce bugs
- Stack frame changes might break existing patterns
- Performance targets might be ambitious

### Mitigation
- Extensive testing at each phase
- Gradual rollout with feature flags
- Performance monitoring throughout development
- Ability to fall back to memory-based allocation

## 🎉 Expected Impact

v0.4.0 "Register Revolution" will transform MinZ from a working compiler into a **high-performance Z80 code generator**. The combination of physical register allocation and stack-based locals will bring MinZ's performance characteristics much closer to hand-optimized assembly while maintaining the benefits of a high-level language.

This release will establish MinZ as a serious tool for Z80 development, capable of generating code that can compete with carefully written assembly for performance-critical applications.