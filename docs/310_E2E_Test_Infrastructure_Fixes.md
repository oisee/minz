# E2E Test Infrastructure Fixes (January 2026)

## Overview

The E2E (end-to-end) test infrastructure in `minzc/pkg/z80testing/` was broken due to several issues accumulated over compiler evolution. This document details the fixes applied to restore test functionality.

## Issues Fixed

### 1. Missing Import: `strings` Package

**Files affected:**
- `e2e_tsmc_verification_test.go`
- `e2e_harness_test.go`

**Problem:** Tests used `strings.Repeat()` and `strings.Contains()` without importing the `strings` package.

**Fix:**
```go
import (
    "strings"
    // ... other imports
)
```

### 2. SMCStats Field Name Mismatches

**File:** `tsmc_benchmarks.go` (lines 837-842)

**Problem:** Code referenced non-existent fields on `SMCStats` struct:
- `UniqueLocations` → should be `UniqueAddresses`
- `CodeModifications` → should be `CodeEvents`
- `DataModifications` → should be `DataEvents`

**Fix:**
```go
// Before (broken)
result.WithTSMC.SMCDetails.UniqueLocations
result.WithTSMC.SMCDetails.CodeModifications
result.WithTSMC.SMCDetails.DataModifications

// After (fixed)
result.WithTSMC.SMCDetails.UniqueAddresses
result.WithTSMC.SMCDetails.CodeEvents
result.WithTSMC.SMCDetails.DataEvents
```

### 3. Outdated Compiler Flags

**File:** `e2e_harness.go` (CompileMinZ function)

**Problem:** Test harness used obsolete compiler flags:
- `-O` (old optimization flag, no longer exists)
- `--enable-true-smc` (old SMC enable flag)

**Current compiler behavior:**
- Optimizations enabled by default
- SMC enabled by default
- Use `--disable-optimize` and `--disable-smc` to turn off

**Fix:**
```go
// Before (broken)
args := []string{
    sourceFile,
    "-o", outputFile,
    "-O",  // No longer exists!
}
if enableTSMC {
    args = append(args, "--enable-true-smc")  // No longer exists!
}

// After (fixed)
args := []string{
    sourceFile,
    "-o", outputFile,
    // Optimizations and SMC enabled by default
}
if !enableTSMC {
    args = append(args, "--disable-smc")  // Disable when not wanted
}
```

### 4. External Assembler Dependency (sjasmplus)

**File:** `e2e_harness.go` (AssembleA80 function)

**Problem:** Tests required external `sjasmplus` assembler at a Mac-specific hardcoded path (`/Users/alice/dev/bin/sjasmplus`), which:
- Doesn't exist on Linux
- Creates external dependency
- Ignores the built-in MZA assembler

**Fix:** Use built-in `z80asm` package:
```go
// Before (broken)
func (h *E2ETestHarness) AssembleA80(a80File string) ([]byte, map[string]uint16, error) {
    cmd := exec.Command(h.sjasmplusPath,
        "--raw="+binFile,
        "--lst="+lstFile,
        "--sym="+labFile,
        a80File,
    )
    // ... external process handling
}

// After (fixed)
func (h *E2ETestHarness) AssembleA80(a80File string) ([]byte, map[string]uint16, error) {
    asm := z80asm.NewAssembler()
    result, err := asm.AssembleFile(a80File)
    if err != nil {
        return nil, nil, fmt.Errorf("assembly failed: %v", err)
    }
    if len(result.Errors) > 0 {
        // ... handle errors
    }
    return result.Binary, result.Symbols, nil
}
```

### 5. Compiler Path Discovery

**File:** `e2e_harness.go` (NewE2ETestHarness function)

**Problem:** Hardcoded compiler path that didn't match actual binary name (`main` vs `minzc`).

**Fix:** Try multiple candidate paths:
```go
candidatePaths := []string{
    filepath.Join(os.Getenv("PWD"), "main"),
    filepath.Join(os.Getenv("PWD"), "minzc"),
    "./main",
    "./minzc",
    "../main",
    "../../main",
}
for _, path := range candidatePaths {
    if _, err := os.Stat(path); err == nil {
        minzcPath = path
        break
    }
}
```

### 6. Invalid Label Names (Dashes and Leading Digits)

**File:** `minzc/pkg/codegen/z80.go` (sanitizeLabel, sanitizeFunctionName, getFunctionLabel)

**Problem:** When compiling from temp directories like `/tmp/minz-e2e-12345/test.minz`, the path components became part of label names:
- Dashes (`-`) in labels are interpreted as subtraction
- Labels starting with digits are invalid

**Example of broken label:**
```asm
; Generated from /tmp/minz-e2e-12345/test.minz
_tmp_minz-e2e-12345_test_sum_range_loop:  ; BROKEN: dash = subtraction!
    JP 12345_test_sum_range_end            ; BROKEN: starts with digit!
```

**Fix:** Sanitize labels in three functions:
```go
func (g *Z80Generator) sanitizeLabel(label string) string {
    funcName := strings.ReplaceAll(g.currentFunc.Name, ".", "_")
    funcName = strings.ReplaceAll(funcName, "$", "_")
    funcName = strings.ReplaceAll(funcName, "-", "_")  // NEW: dashes
    // Ensure doesn't start with digit
    if len(funcName) > 0 && funcName[0] >= '0' && funcName[0] <= '9' {
        funcName = "_" + funcName
    }
    return fmt.Sprintf("%s_%s_i%d", funcName, label, g.inlineCounter)
}
```

### 7. Wrong Function Keyword (`fn` vs `fun`)

**Files:** All test files with embedded MinZ source code
- `e2e_harness_test.go`
- `e2e_tsmc_verification_test.go`
- `tsmc_verification_test.go`
- `tsmc_benchmarks.go`

**Problem:** Test source code used `fn` keyword, but MinZ uses `fun` (because programming should be FUN!).

**Fix:** Global replace in test files:
```bash
sed -i 's/\bfn /fun /g' pkg/z80testing/*.go
```

### 8. Symbol Name Case Sensitivity

**File:** `e2e_harness_test.go`

**Problem:** Tests looked for symbol `"add"` but MinZ generates uppercase mangled names like `"ADD_U16_U16"`.

**Fix:** Case-insensitive symbol search:
```go
// Before (broken)
if _, ok := symbols["add"]; !ok {
    t.Error("Symbol 'add' not found")
}

// After (fixed)
foundAdd := false
for sym := range symbols {
    if strings.Contains(strings.ToLower(sym), "add") {
        foundAdd = true
        break
    }
}
```

## Test Results

After all fixes, the basic E2E test passes:

```
=== RUN   TestE2EHarnessBasic
=== RUN   TestE2EHarnessBasic/compile_without_TSMC
    e2e_harness_test.go:80: Found add function symbol: ADD_U16_U16
=== RUN   TestE2EHarnessBasic/compile_with_TSMC
    e2e_harness_test.go:112: Found add function symbol: ADD_U16_U16_PARAM_B$IMM0
--- PASS: TestE2EHarnessBasic (0.10s)
    --- PASS: TestE2EHarnessBasic/compile_without_TSMC (0.05s)
    --- PASS: TestE2EHarnessBasic/compile_with_TSMC (0.05s)
PASS
```

## Files Modified

| File | Changes |
|------|---------|
| `pkg/z80testing/e2e_harness.go` | Built-in assembler, flexible paths, fixed flags |
| `pkg/z80testing/e2e_harness_test.go` | Added strings import, case-insensitive symbols, fn→fun |
| `pkg/z80testing/e2e_tsmc_verification_test.go` | Added strings import, fn→fun |
| `pkg/z80testing/tsmc_verification_test.go` | fn→fun |
| `pkg/z80testing/tsmc_benchmarks.go` | Fixed SMCStats fields, fn→fun |
| `pkg/codegen/z80.go` | Label sanitization (dashes, digits) |

## Lessons Learned

1. **Keep tests in sync with compiler evolution** - Flag changes broke tests silently
2. **Avoid external dependencies in tests** - Use built-in tools when available
3. **Sanitize ALL special characters in labels** - Not just dots and $, but also dashes
4. **Test with diverse directory names** - Including those with numbers and special chars
5. **Document language keywords clearly** - `fun` not `fn`!

---

*Part of MinZ v0.16.5 maintenance*
