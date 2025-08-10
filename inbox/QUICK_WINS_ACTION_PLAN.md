# 🎯 MinZ Compiler Quick Wins Action Plan

**Date**: 2025-08-10  
**Status**: In Progress  
**Goal**: Improve compilation success rate from 56% to 70%+

## 📊 Current State Analysis

### Success Metrics
- **Overall**: 56% (50/88 examples)
- **Basic Language**: 83% (15/18) - 3 failures
- **Data Structures**: 67% (8/12) - 4 failures  
- **Advanced Features**: 60% (12/20) - 8 failures

## 🚀 Quick Wins Identified

### 1. Fix Basic Language Failures (Impact: +3% success rate)

#### ❌ stdlib_basic_test.minz
**Error**: `undefined function: print_i8`
**Fix**: Add print_i8 to builtin functions
**Effort**: 5 minutes
**File**: `minzc/pkg/semantic/builtin_functions.go`

#### ❌ math_functions.minz
**Error**: `cannot determine type of expression being cast`
**Issue**: `(-x) as u8` where x is i8
**Fix**: Handle unary negation type inference for signed types
**Effort**: 30 minutes
**File**: `minzc/pkg/semantic/analyzer.go`

#### ❌ pub_fun_example.minz
**Error**: `undefined function: score_manager.add_points`
**Issue**: Method syntax not supported on non-interface types
**Fix**: Add error message suggesting function call syntax
**Effort**: 15 minutes

### 2. Multiplication Optimization Not Working (Impact: Performance)

**Discovery**: Multiplication by constants uses loops instead of bit-shifts
```asm
; Current: Loop multiplication (200+ T-states)
mul_loop:
    ADD HL, DE
    DEC C
    JR NZ, mul_loop

; Expected: Bit-shift optimization (30-50 T-states)  
SLA A  ; x * 2
```

**Root Cause**: Optimization not implemented in codegen
**Fix Location**: `minzc/pkg/codegen/z80/codegen.go`
**Effort**: 2-4 hours

### 3. Dead Code Elimination Too Aggressive (Impact: Testing)

**Issue**: Optimizer removes entire functions when results unused
**Example**: `test_multiplications()` completely eliminated
**Fix**: Add `@keep` annotation or compiler flag
**Effort**: 1 hour

## 📝 Implementation Priority

### Phase 1: Immediate Fixes (1 hour)
1. ✅ Add `print_i8` to builtins
2. ✅ Fix unary negation type inference  
3. ✅ Better error for method syntax on non-interfaces

### Phase 2: Core Improvements (4 hours)
1. ⏳ Implement multiplication optimization
2. ⏳ Add @keep annotation for testing
3. ⏳ Fix dead code elimination flags

### Phase 3: Documentation (2 hours)
1. ⏳ Update examples README with fixed counts
2. ⏳ Document optimization patterns
3. ⏳ Create troubleshooting guide

## 🎯 Expected Outcomes

### Success Rate Improvements
- Basic Language: 83% → **100%** (+3 examples)
- Overall: 56% → **59%** minimum

### Performance Gains
- Multiplication: **3-18x faster**
- Code size: Minimal increase
- Runtime: Significant improvement

## 💡 Additional Quick Wins

### 1. Error Message Improvements
- Add "did you mean?" suggestions
- Show available functions when undefined
- Highlight type mismatches clearly

### 2. Debugging Enhancements  
- Add --keep-all flag to prevent optimization
- Implement --dump-mir for all functions
- Create --explain-optimization flag

### 3. Standard Library Stubs
- Create minimal stdlib with common functions
- Add print variants for all basic types
- Implement basic math functions

## 📈 Success Metrics

### Before Quick Wins
- 56% compilation success
- 3 basic examples failing
- No multiplication optimization
- Aggressive dead code elimination

### After Quick Wins
- 59%+ compilation success
- 0 basic examples failing
- 3-18x faster multiplication
- Controllable optimization

## 🔧 Implementation Steps

### Step 1: Fix print_i8 (5 minutes)
```go
// In builtin_functions.go
"print_i8": {
    Name: "print_i8",
    Params: []ir.Type{&ir.BasicType{Kind: ir.TypeI8}},
    ReturnType: &ir.BasicType{Kind: ir.TypeVoid},
}
```

### Step 2: Fix Unary Negation (30 minutes)
```go
// In analyzer.go, analyzeUnaryOp
case "-":
    if basicType, ok := exprType.(*ir.BasicType); ok {
        if basicType.Kind == ir.TypeI8 || basicType.Kind == ir.TypeI16 {
            // Negation preserves signed type
            return &ir.UnaryOp{Op: "-", Operand: expr, Type: exprType}, nil
        }
    }
```

### Step 3: Multiplication Optimization (2-4 hours)
```go
// In z80/codegen.go
func (g *Generator) generateMultiplication(left, right ir.Value) {
    if constVal := getConstant(right); constVal != nil {
        switch constVal.Value {
        case 2:
            g.emit("SLA A  ; x * 2")
            return
        case 10:
            g.generateMultiplyBy10(left)
            return
        }
    }
    // Fall back to loop multiplication
}
```

## 🚦 Next Actions

1. **Immediate**: Fix the 3 basic language failures
2. **Today**: Implement multiplication optimization
3. **Tomorrow**: Update documentation and tests
4. **This Week**: Release v0.10.2 with improvements

## 📊 Tracking Progress

- [x] Document quick wins
- [ ] Fix print_i8 builtin
- [ ] Fix unary negation type inference
- [ ] Improve method syntax error
- [ ] Implement multiplication optimization
- [ ] Add @keep annotation
- [ ] Update success metrics
- [ ] Release improvements

---

**Status**: Ready for implementation  
**Priority**: HIGH - Low effort, high impact improvements  
**Timeline**: Complete within 24 hours