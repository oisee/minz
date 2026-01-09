# Known Semantic Issues - January 2026

## Summary

4 examples fail due to semantic analyzer bugs. Parser works 100%.

## Issue #1: Pointer Arithmetic Type Tracking

**Affected Examples:** `pointer_arithmetic.minz`, `nested_loops.minz`

**Error:** `cannot dereference non-pointer type: u16`

**Code Pattern:**
```minz
fun array_sum(data: *u8, count: u8) -> u16 {
    while i < count {
        sum = sum + (*data as u16);  // ERROR: data is seen as u16, not *u8
        data = data + 1;              // This assignment might corrupt type
        i = i + 1;
    }
}
```

**Analysis:**
- Parameter `data` is declared as `*u8`
- After `data = data + 1`, the type should still be `*u8`
- But semantic analyzer tracks it as `u16`
- Likely issue: expression type caching or symbol table update bug

**Fix Complexity:** Medium - needs investigation of type tracking through assignments

---

## Issue #2: Module Import Resolution

**Affected Examples:** `cpm_hello.minz`

**Error:** `undefined function: putchar`

**Code Pattern:**
```minz
import cpm.bdos;  // Module exists at stdlib/cpm/bdos.minz

fun main() -> void {
    putchar('H');  // ERROR: putchar not found
}
```

**Analysis:**
- The module `stdlib/cpm/bdos.minz` exists and contains `putchar`
- The `import cpm.bdos` statement isn't resolving correctly
- Likely issue: import path resolution or symbol export mechanism

**Fix Complexity:** Medium - needs import system debugging

---

## Issue #3: Loop Iterator Type Declaration

**Affected Examples:** `loops_indexed.minz`

**Error:** `undefined type: idx`

**Code Pattern:**
```minz
loop scores indexed to score, idx {  // idx should auto-declare as u8
    if idx == max_index {            // ERROR: idx is undefined
        // ...
    }
}
```

**Analysis:**
- `loop ... indexed to ..., idx` syntax auto-declares `idx`
- The semantic analyzer doesn't create the symbol for `idx`
- Likely issue: iterator pattern handling incomplete

**Fix Complexity:** Medium - needs iterator analysis code update

---

## Priority Assessment

| Issue | Impact | Effort | Priority |
|-------|--------|--------|----------|
| Pointer Type Tracking | High | High | P2 |
| Module Import | Medium | Medium | P2 |
| Loop Iterator | Low | Low | P3 |

**Recommendation:** Focus on DAP debugger first. These issues affect edge cases but don't block most development. Fix them incrementally as we work on the Q1 roadmap.

---

## Current Compilation Stats

```
Total Examples:    53
Parse Success:     53 (100%)
Compile Success:   49 (92.4%)
Semantic Failures: 4 (7.6%)
```

## Quick Wins Already Applied

1. ✅ Fixed copyPropagation infinite loop
2. ✅ Fixed missing opcode String() representations
3. ✅ Added plasma demo examples
4. ✅ Documented 2026 vision and roadmap

## Next Phase: DAP Debugger

These semantic issues will become easier to debug once we have a working debugger. The DAP integration is higher priority because:

1. It enables debugging of all existing working examples
2. It provides infrastructure for fixing remaining issues
3. It's a major step toward v1.0 release
