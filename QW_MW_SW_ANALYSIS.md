# QW-MW-SW Analysis and Implementation Plan

## 🎯 Quick Wins (1-2 days each)

### QW1: Remove :: Enum Notation ✅ READY
**Impact:** Improves syntax consistency  
**Effort:** 2 hours  
**Files affected:** ~10-15 test files  
**Implementation:**
```minz
// Old: State::Playing
// New: State.Playing
```
**Plan:** Simple find/replace across examples

### QW2: Document Module Import System ✅ READY
**Impact:** Clarifies existing functionality  
**Effort:** 1 hour  
**Files affected:** Documentation only  
**Implementation:** Write clear guide for module imports that already work

### QW3: Fix Parameter Scope Bug 🔴 CRITICAL
**Impact:** Fixes 4 failing tests, enables tail recursion  
**Effort:** 4-6 hours  
**Files affected:** `pkg/codegen/z80.go`  
**Root cause:** Parameters lost during recursive calls  
**Solution:** Maintain parameter registry through function scope

### QW4: Suppress Optimizer Noise ✅ READY
**Impact:** Cleaner output  
**Effort:** 30 minutes  
**Implementation:** Check MINZ_QUIET env var in tail recursion optimizer

### QW5: Fix Missing Built-ins ✅ READY
**Impact:** Fixes 1 test  
**Effort:** 1 hour  
**Built-ins needed:** `pad()`, `format()`

## 🔨 Medium Wins (3-5 days each)

### MW1: Complete Error Propagation (?? operator)
**Impact:** Modern error handling  
**Effort:** 3 days  
**Status:** Partially implemented  
**Remaining:**
- Implement `??` null coalescing
- Fix error type inference
- Add proper unwrapping

### MW2: Self Parameter & Method Calls
**Impact:** Natural OOP syntax  
**Effort:** 4 days  
**Example:** `circle.draw()` instead of `draw(circle)`  
**Tasks:**
- Parse self parameter
- Implement method dispatch
- Handle field access in methods

### MW3: Fix Pattern Matching Nil
**Impact:** Fixes 2 critical tests  
**Effort:** 2 days  
**Problem:** Enum patterns create nil AST nodes  
**Solution:** Proper enum member parsing in case statements

## 🚀 Strategic Wins (1-2 weeks each)

### SW1: Generic Functions
**Impact:** Type-safe reusable code  
**Effort:** 2 weeks  
**Example:** `fun map<T, U>(items: [T], f: fn(T) -> U) -> [U]`  
**Complex:** Requires type system overhaul

### SW2: Local/Nested Functions
**Impact:** Functional programming patterns  
**Effort:** 1 week  
**Example:** Closures, higher-order functions  
**Tasks:** Closure capture, function types

### SW3: Complete Pattern Matching
**Impact:** ML-style programming  
**Effort:** 2 weeks  
**Features:** Guards, destructuring, exhaustiveness

## Priority Matrix

```
         High Impact
              ↑
    QW3 ━━━━━┃━━━━━ MW3
    (param)  ┃   (pattern)
             ┃
    QW1 ━━━━━┃━━━━━ MW1
    (enum)   ┃   (error ??)
             ┃
    QW2 ━━━━━┃━━━━━ MW2
    (docs)   ┃   (methods)
    ━━━━━━━━━┃━━━━━━━━━━→
    Quick    ┃    Long
            Effort
```

## Recommended Execution Order

### Phase 1: Quick Wins (This Session)
1. ✅ QW1: Remove :: notation (30 min)
2. ✅ QW4: Suppress optimizer noise (30 min)
3. ✅ QW2: Document imports (1 hour)
4. 🔧 QW3: Fix parameter scope (4 hours)

### Phase 2: Critical Fixes (Next Session)
5. MW3: Fix pattern matching nil
6. MW1: Complete ?? operator

### Phase 3: Feature Completion (Future)
7. MW2: Method calls
8. SW2: Nested functions
9. SW1: Generics

## Expected Impact

After Quick Wins:
- **Success rate: 67% → 72%** (+5%)
- Cleaner output
- Better documentation

After Medium Wins:
- **Success rate: 72% → 80%** (+8%)
- Modern error handling
- Natural OOP syntax

After Strategic Wins:
- **Success rate: 80% → 90%** (+10%)
- Full functional programming
- Type-safe generics