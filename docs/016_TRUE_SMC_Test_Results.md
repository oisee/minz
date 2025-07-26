# TRUE SMC Test Results and Analysis

**Version:** 0.2.5  
**Date:** July 26, 2025  
**Status:** Initial Testing Complete

---

## 1. Test Summary

### Overall Results
- **TRUE SMC anchor generation**: ✅ Working
- **Anchor reuse**: ⚠️ Partially working (bug found)
- **Call-site patching**: ❌ Not implemented
- **16-bit support**: ✅ Working (anchors created)
- **DI/EI protection**: ❌ Not implemented

### Grade: C- (Acceptable with significant issues)

---

## 2. Detailed Test Results

### Test 1: Basic 8-bit Parameters

**Input:**
```minz
fn add(x: u8, y: u8) -> u8 {
    return x + y
}
```

**Generated Assembly:**
```asm
add:
x$imm0:
    LD A, 0        ; x anchor (will be patched) ✅
    LD ($F000), A
y$imm0:
    LD A, 0        ; y anchor (will be patched) ✅
    LD ($F002), A
    LD A, (x$imm0)    ; Reuse from anchor ✅
    LD ($F006), A
    LD A, (x$imm0)    ; ❌ BUG: Should load y$imm0
    LD ($F008), A
```

**Issues Found:**
1. Parameter y is not being reused correctly
2. No patching logic at call sites

**Score: 55/100**

### Test 2: Function with Direct Return

**Status:** ✅ Best performing test

**Generated Assembly:**
```asm
calculate_once:
a$imm0:
    LD A, 0        ; a anchor ✅
    LD ($F000), A
b$imm0:
    LD A, 0        ; b anchor ✅
    LD ($F002), A
    LD A, (a$imm0)    ; Reuse from anchor ✅
    LD ($F006), A
    ; Direct return optimization also working
    LD (result), HL
```

This test showed the most complete TRUE SMC implementation.

---

## 3. Bug Analysis

### Bug #1: Incorrect Parameter Reuse
**Severity:** High
**Location:** `generateTrueSMCFunction` in codegen/z80.go

The function is storing all parameters to sequential virtual registers but not tracking which parameter maps to which anchor. When generating reuse code, it's incorrectly referencing the wrong anchor.

**Expected:**
```asm
LD A, (y$imm0)    ; Load y parameter
```

**Actual:**
```asm
LD A, (x$imm0)    ; Incorrectly loading x again
```

### Bug #2: Missing Call-Site Patching
**Severity:** Critical
**Location:** `generateTrueSMCCall` in codegen/z80.go

The patching logic is generating placeholder code:
```asm
LD A, <arg0>      ; Placeholder, not actual value
```

This needs to:
1. Load actual argument values
2. Patch the anchor addresses before CALL

---

## 4. What's Working Well

1. **Anchor Creation**: Parameter anchors are correctly created with immediate operands
2. **Anchor Symbols**: Proper naming convention (param$imm0)
3. **TRUE SMC Function Detection**: Functions are correctly identified and processed
4. **Basic Structure**: The overall approach matches the specification

---

## 5. Comparison: Ideal vs Actual

### Ideal TRUE SMC for add(5, 3):

```asm
; Function definition
add:
x$imm0:
    LD A, 0        ; x anchor
y$imm0:  
    LD A, 0        ; y anchor
    ADD A, (y$imm0) ; Direct addition
    RET

; Call site
    LD A, 5
    LD (x$imm0+1), A
    LD A, 3
    LD (y$imm0+1), A
    CALL add
```

### Current Output:
- ✅ Anchors created correctly
- ❌ Inefficient parameter handling (too many stores/loads)
- ❌ No patching at call site
- ❌ Bug in parameter reuse

---

## 6. Performance Analysis

Current implementation overhead:
- Each parameter: 2 instructions (anchor + store)
- Each reuse: 2 instructions (load from anchor + store)
- Missing optimization: Direct use of anchors in operations

Potential improvements:
- Use anchors directly in arithmetic operations
- Eliminate intermediate stores where possible
- Implement proper call-site patching

---

## 7. Recommendations

### Immediate Fixes Required:

1. **Fix parameter mapping bug**
   - Track parameter index to anchor symbol mapping
   - Ensure correct anchor is referenced for each parameter use

2. **Implement call-site patching**
   - Generate actual argument loads
   - Patch anchor addresses before CALL
   - Add DI/EI for 16-bit patches

3. **Optimize anchor usage**
   - Allow direct use of anchor values in operations
   - Reduce intermediate register stores

### Future Enhancements:

1. **Recursive function support** with SMC undo-log
2. **Better CFG analysis** for optimal anchor placement
3. **Diagnostic reporting** (-report-smc-anchors flag)

---

## 8. Test Suite Recommendations

Create focused unit tests for:
1. Single parameter functions
2. Multiple parameter functions  
3. Parameter reuse patterns
4. 16-bit parameter handling
5. Call-site patching verification

Each test should verify:
- Anchor creation at first use
- Correct anchor reuse
- Proper patching before calls
- No duplicate anchors

---

## 9. Conclusion

TRUE SMC implementation is partially working but requires critical fixes:
- Core concept is implemented correctly
- Anchor generation works as specified
- Major bugs in parameter reuse and call-site patching
- Performance benefits not fully realized due to inefficiencies

**Next Steps:**
1. Fix the parameter reuse bug
2. Implement proper call-site patching
3. Add comprehensive test coverage
4. Optimize generated code for TRUE SMC benefits

The foundation is solid, but the implementation needs refinement to achieve the full performance benefits promised by TRUE SMC.