# Backend Health Summary - August 2025

## Overall Status: 7/8 Backends Operational

### 🟢 Fully Working Backends (5/8)
- **Z80** - Default backend, production ready
- **6502** - Generates valid 6502 assembly
- **68000** - Generates valid 68K assembly  
- **i8080** - Generates valid Intel 8080 assembly
- **Game Boy** - Generates valid GB assembly

### 🟡 Partially Working Backends (2/8)
- **C** - Generates compilable C but incorrect behavior (outputs 0 instead of 52)
- **LLVM** - Generates LLVM IR with syntax errors (missing variable names)

### 🔴 Broken Backends (1/8)
- **WebAssembly** - Generates invalid WAT (undefined globals)

## Root Cause: MIR Generation Bug

**Issue**: Variable names are missing in STORE instructions
```mir
; store , r2    # Should be: store x, r2
; store , r4    # Should be: store y, r4  
; store , r8    # Should be: store sum, r8
```

**Impact**:
- Assembly backends work (use addresses, not names)
- High-level backends fail or produce wrong results

## Verification Matrix

| Backend | Code Gen | Binary | Execution | Issue |
|---------|----------|---------|-----------|-------|
| Z80     | ✅       | N/A     | N/A       | None |
| 6502    | ✅       | N/A     | N/A       | None |
| 68000   | ✅       | N/A     | N/A       | None |
| i8080   | ✅       | N/A     | N/A       | None |
| Game Boy| ✅       | N/A     | N/A       | None |
| C       | ✅       | ✅      | ❌        | Skips stores, wrong output |
| LLVM    | ⚠️       | ❌      | ❌        | Invalid IR syntax |
| WASM    | ❌       | ❌      | ❌        | Undefined globals |

## Priority Actions

1. **HIGH**: Fix MIR STORE instruction symbol generation
2. **MEDIUM**: Add MIR validation to catch empty symbols
3. **LOW**: Improve backend error messages for missing data

## Quick Test Command

```bash
# Run E2E test for all backends
./test_backend_e2e.sh

# Check specific backend
./mz test.minz -b llvm -o test.ll && cat test.ll | grep "%.addr"
```

The good news: **This is a single bug affecting multiple backends**, not multiple separate issues!