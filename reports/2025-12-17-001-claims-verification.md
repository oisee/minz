# MinZ Claims Verification Report

**Date:** 2025-12-17
**Evaluator:** Claude Code Assessment
**Version Tested:** v0.15.0-dev (master branch)

---

## Executive Summary

This report verifies claims made in the MinZ repository documentation (README.md, STATUS.md, CLAUDE.md) against actual compiler behavior. **Significant discrepancies** were found between documented capabilities and tested functionality.

### Key Findings

| Metric | Claimed | Actual | Status |
|--------|---------|--------|--------|
| Compilation Success Rate | 88% (150/170) | **81% (47/58)** | Close but overstated |
| Error Propagation (`?`) | Working | **NOT WORKING** | Major gap |
| Pattern Matching | Working | **NOT WORKING** | Major gap |
| Lambda Iterator Chains | DJNZ optimized | **Unoptimized** | Misleading |
| Function Pointers | Working | **NOT WORKING** | Major gap |
| Module Imports | Working | **Partial** | Library missing |
| Array Literals → DB | "Smart Arrays" | **Incomplete** | TODO in output |

---

## 1. Compilation Success Rate

### README Claim
> "✅ **88%** of examples compile successfully (150/170)"

### Test Results
```
Success: 47/58 examples = 81%
```

**Failed Examples (11):**
- `asm_mir_functions.minz`
- `local_functions_test.minz`
- `mnist_complete.minz`, `mnist_simple.minz`
- `smc_optimization.minz`, `smc_optimization_simple.minz`
- `stdlib_metafunction_test.minz`
- `true_smc_lambdas.minz`
- `zero_cost_interfaces.minz`, `zero_cost_interfaces_test.minz`, `zero_cost_test.minz`

**Verdict:** OVERSTATED (81% vs 88%)

### Critical Setup Issue Found
Tree-sitter warning messages were not being filtered from output, causing 0% success rate without proper tree-sitter configuration. Users must run:
```bash
tree-sitter init-config
```
This is **not documented** in installation instructions.

---

## 2. Feature-by-Feature Verification

### 2.1 Error Propagation (`?` operator)

**README Claim:**
```minz
fun read_file?(path: *u8) -> *u8 ? Error {
    let file = open(path)?;  // Propagate errors with ?
    ...
}
```

**Test:**
```minz
fun risky_op?(x: u8) -> u8 ? Error {
    if x == 0 { return @error("zero value"); }
    return x * 2;
}
```

**Result:** FAILS
```
Error: invalid error type for function risky_op: undefined type: Error
```

**Verdict:** NOT IMPLEMENTED - Error type and `?` operator don't exist

---

### 2.2 Pattern Matching

**README Claim:**
```minz
case parse_input() {
    Ok(n) => process(n),
    Error(0) => @print("Invalid input"),
}
```

**Test:**
```minz
case r {
    Ok => return 1,
    Error => return 0
}
```

**Result:** FAILS
```
Error: undefined identifier 'return'
```

**Verdict:** NOT IMPLEMENTED - Parser doesn't handle case statements properly

---

### 2.3 Lambda/Function Pointers

**README Claim:**
> "Zero-cost lambdas" and higher-order functions

**Test:**
```minz
fun apply(f: fn(u8) -> u8, x: u8) -> u8 {
    return f(x);
}
```

**Result:** FAILS
```
Error: undefined function: f
Error: cannot use double as value
```

**Verdict:** NOT IMPLEMENTED - Cannot pass functions as values

---

### 2.4 Iterator Chains → DJNZ Optimization

**README Claim:**
```minz
numbers.iter()
    .filter(|x| x > 5)
    .map(|x| x * 2)
    .forEach(|x| print(x));  // Compiles to optimal DJNZ loop!
```

**Test Result:**
Code compiles but generates **individual lambda functions**, NOT optimized DJNZ loops.

Output shows:
- 3 separate `iterator_main_X` functions
- No DJNZ instruction
- No loop fusion

**Verdict:** MISLEADING - Compiles but doesn't optimize as claimed

---

### 2.5 Array Literals → DB Directives

**README Claim:**
```minz
let data: [u8; 5] = [10, 20, 30, 40, 50];
// Generates: DB 10, 20, 30, 40, 50
```

**Actual Output:**
```asm
; Array literal [[10 20 30 40 50]] -> DB directive
LD HL, array_data_0
...
LD (HL), 0    ; TODO: Need value source
```

**Verdict:** INCOMPLETE - Has TODO comment, doesn't generate DB directive

---

### 2.6 Module System

**README Claim:**
```minz
import math as m;
let x = m.abs(-5);
```

**Test Result:**
```
Error: undefined function: math.abs
```

**Verdict:** PARTIAL - Import syntax parses but stdlib not found

---

### 2.7 Working Features (Verified)

These features **do work** as claimed:

| Feature | Test | Result |
|---------|------|--------|
| Basic functions | `fun add(a: u8, b: u8) -> u8` | Works |
| Struct literals | `Point { x: 10, y: 20 }` | Works |
| Field access | `p.x + p.y` | Works |
| @print with interpolation | `@print("Hello #{NAME}")` | Works |
| CTIE (@ctie) | Compile-time execution | Works |
| For loops | `for i in 0..10` | Works |
| While loops | Basic while | Works |
| Global variables | `global x: u8 = 0` | Works |
| Constants | `const X = 5` | Works |

---

## 3. Toolchain Status

### 3.1 Assembler (mza)
- **Claim:** Self-contained Z80 assembler
- **Status:** Works but pre-built binary was wrong architecture (macOS ARM64 on Linux)
- **After rebuild:** Functions correctly

### 3.2 Emulator (mze)
- **Claim:** 100% Z80 instruction coverage
- **Status:** Claims verified via help output; uses remogatto/z80 library
- **Note:** Pre-built binary had wrong architecture

### 3.3 Crystal Backend
- **Claim:** Full Crystal code generation
- **Status:** Generates code but incomplete (missing struct definitions)

---

## 4. Documentation Inconsistencies

### Version Confusion
- README.md: v0.15.0
- STATUS.md: v0.10.0 "Lambda Revolution" (dated 2025-08-09)
- FEATURE_STATUS.md: v0.12.0 Alpha

### Success Rate Confusion
- README: 88% (150/170 examples)
- STATUS.md: 75-80%
- FEATURE_STATUS.md: 69% (44/63 files)
- **Actual test:** 81% (47/58 files in examples/)

### Feature Claims Confusion
- STATUS.md claims "Error propagation complete" - **FALSE**
- STATUS.md claims "Lambda expressions zero-cost" - **PARTIAL** (compiles but unoptimized)
- FEATURE_STATUS.md correctly marks many features as partial/not implemented

---

## 5. Priority Fixes for Honesty

### Immediate Documentation Fixes

1. **Remove error propagation claims** from README until implemented
2. **Remove pattern matching examples** from README until working
3. **Remove "DJNZ optimization" claims** - iterator chains don't optimize
4. **Update success rate** to actual 81%
5. **Add tree-sitter setup** to installation instructions
6. **Fix version numbers** - use consistent versioning

### Code Fixes (Priority Order)

| Priority | Issue | Effort | Impact |
|----------|-------|--------|--------|
| P0 | Document tree-sitter init requirement | 5 min | Unblocks all users |
| P1 | Filter ANSI codes from tree-sitter output | 1 hour | Reliability |
| P2 | Array literal DB generation | 2-4 hours | Match claims |
| P3 | Basic pattern matching | 1-2 days | Core feature |
| P4 | Error propagation | 2-3 days | Core feature |
| P5 | Function pointer passing | 1-2 days | Core feature |

---

## 6. Recommendations

### Short Term (This Week)
1. Update README to be honest about current state
2. Add tree-sitter setup to installation docs
3. Fix pre-built binaries architecture issue
4. Update STATUS.md with current date and actual metrics

### Medium Term (Next Month)
1. Complete array literal → DB directive generation
2. Implement basic pattern matching
3. Add function pointer support
4. Create automated test suite with actual success tracking

### Long Term
1. Implement error propagation system
2. Add iterator chain optimization (DJNZ fusion)
3. Complete stdlib modules
4. Crystal backend struct support

---

## Appendix A: Test Commands

```bash
# Verify compilation success rate
success=0 && fail=0 && for f in examples/*.minz; do
    if ./minzc/main "$f" -o /tmp/out.a80 2>/dev/null; then
        success=$((success + 1))
    else
        fail=$((fail + 1))
    fi
done
echo "Success: $success, Fail: $fail"

# Test specific features
cat > /tmp/test.minz << 'EOF'
// your test code
EOF
./minzc/main /tmp/test.minz -o /tmp/test.a80
```

---

*Report generated by systematic testing of all documented claims against actual compiler behavior.*
