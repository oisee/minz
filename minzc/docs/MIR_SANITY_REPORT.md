# MIR Sanity Check Report

**Date:** 2026-01-09
**Status:** Analysis Complete
**Author:** Claude Code

## Executive Summary

The MIR (MinZ Intermediate Representation) layer has been analyzed for completeness. While the VM executes basic programs correctly, there are significant gaps between what the compiler generates and what the MIR parser can handle.

**Key Finding:** The MIR round-trip (Semantic → String() → Parse → Execute) only works for simple programs. Arrays, structs, and function parameters break the pipeline.

## Architecture Overview

```
MinZ Source
    ↓
Semantic Analyzer (generates 79 opcodes)
    ↓
MIR Backend (calls inst.String() for text output)
    ↓
MIR Parser (pattern matching, ~35 opcodes)
    ↓
MIR VM (executes ~49 opcodes)
    ↓
Output (PNG via syscalls)
```

## Coverage Analysis

### By Numbers

| Stage | Opcodes Handled | Percentage |
|-------|-----------------|------------|
| Semantic Analyzer generates | 79 | 100% |
| String() can serialize | ~65 | 82% |
| MIR Parser can parse | ~35 | 44% |
| MIR VM can execute | 49 | 62% |

### Effective Coverage

For **simple programs** (loops, arithmetic, variables, syscalls): **100% working**

For **real programs** (arrays, structs, function calls): **BROKEN**

## Detailed Findings

### 1. Interface Opcodes - NOT NEEDED

| Opcode | Usage in Semantic | Verdict |
|--------|-------------------|---------|
| OpInterfaceCall | 0 | Dead code |
| OpMethodDispatch | 0 | Dead code |
| OpCastInterface | 0 | Dead code |
| OpCheckCast | 0 | Dead code |

**Reason:** MinZ uses monomorphization and zero-cost interfaces (direct calls, no vtables).

**Recommendation:** Remove from ir.go or mark as deprecated.

### 2. Array Opcodes - PARSER GAP

| Opcode | String() Output | Parser Support |
|--------|-----------------|----------------|
| OpLoadIndex | `r%d = r%d[r%d]` | NO |
| OpStoreIndex | `r%d[r%d] = r%d` | NO |
| OpLoadElement | `r%d = r%d[%d]` | NO |
| OpStoreElement | `r%d[%d] = r%d` | NO |
| OpArrayInit | `r%d = array_init(%d)` | NO |
| OpArrayElement | `r%d[%d] = %d` | NO |

**Test Case:**
```minz
let arr: [u8; 5];
arr[0] = 42;
```

**Generated MIR:**
```
r9 = r7[r8]  // Parser cannot handle bracket syntax
```

### 3. Struct Opcodes - PARSER GAP

| Opcode | String() Output | Parser Support |
|--------|-----------------|----------------|
| OpLoadField | `r%d = r%d.field[%d]` | NO |
| OpStoreField | `r%d.field[%d] = r%d` | NO |
| OpLoadBitField | `r%d = r%d.bits[%d:%d]` | NO |
| OpStoreBitField | `r%d.bits[%d:%d] = r%d` | NO |

**Test Case:**
```minz
struct Point { x: u8, y: u8 }
let p: Point;
p.x = 10;
```

**Generated MIR:**
```
r3.field[0] = r2  // Parser cannot handle .field[] syntax
```

### 4. Function Parameter Opcodes - PARSER GAP

| Opcode | String() Output | Parser Support |
|--------|-----------------|----------------|
| OpLoadParam | `r%d = param %s` | NO |

**Test Case:**
```minz
fun add(a: u8, b: u8) -> u8 { return a + b; }
```

**Generated MIR:**
```
unknown op 33  // OpLoadParam not serializing correctly
```

### 5. SMC/TSMC Opcodes - Z80 BACKEND ONLY

| Opcode | Usage |
|--------|-------|
| OpSMCLoadConst | Z80 codegen only |
| OpSMCStoreConst | Z80 codegen only |
| OpTrueSMCLoad | Z80 codegen only |
| OpTrueSMCPatch | Z80 codegen only |
| OpTSMCRef* (5 ops) | Z80 codegen only |

**Verdict:** Correctly not needed in MIR VM. These are target-specific optimizations.

### 6. Memory/Pointer Opcodes - PARTIAL

| Opcode | String() Output | Parser | VM |
|--------|-----------------|--------|-----|
| OpLoad | `r%d = *r%d` | NO | NO |
| OpStore | `*r%d = r%d` | Partial | NO |
| OpAlloc | `r%d = alloc(%d)` | NO | NO |
| OpFree | `free(r%d)` | NO | NO |
| OpMemcpy | `memcpy(...)` | NO | NO |

### 7. Error Handling Opcodes - NOT IMPLEMENTED

| Opcode | String() Output | Status |
|--------|-----------------|--------|
| OpSetError | `set_error r%d` | Not used |
| OpCheckError | `r%d = check_error` | Not used |

**Note:** Error propagation (`?` operator) is documented as NOT IMPLEMENTED in CLAUDE.md.

## Working Features

The following work correctly end-to-end:

| Category | Opcodes |
|----------|---------|
| Arithmetic | Add, Sub, Mul, Div, Mod, Neg, Inc, Dec |
| Bitwise | And, Or, Xor, Not, Shl, Shr |
| Comparison | Lt, Gt, Eq, Ne, Le, Ge |
| Control Flow | Jump, JumpIf, JumpIfNot, Label, Return, DJNZ |
| Variables | LoadVar, StoreVar, LoadConst, LoadImm |
| I/O | Syscall, Print, PrintChar |
| Stack | Push, Pop |

## Recommendations

### Phase 1: Critical Fixes (Parser Alignment)

1. **Add bracket syntax parsing** for array access
   - Parse `r%d = r%d[r%d]` as OpLoadIndex
   - Parse `r%d[r%d] = r%d` as OpStoreIndex

2. **Add .field[] syntax parsing** for struct access
   - Parse `r%d = r%d.field[%d]` as OpLoadField
   - Parse `r%d.field[%d] = r%d` as OpStoreField

3. **Add param keyword parsing**
   - Parse `r%d = param %s` as OpLoadParam

4. **Fix OpLoadParam serialization** in String()

### Phase 2: VM Handlers

Add VM execution handlers for:
- OpLoadIndex, OpStoreIndex
- OpLoadElement, OpStoreElement
- OpLoadField, OpStoreField
- OpLoadParam
- OpArrayInit

### Phase 3: Cleanup

1. Remove dead interface opcodes or mark deprecated
2. Document SMC opcodes as Z80-specific
3. Add tests for all parser patterns

## Test Matrix

| Test Case | Current Status | After Fix |
|-----------|----------------|-----------|
| While loop | PASS | PASS |
| For loop | PASS | PASS |
| Nested loops | PASS | PASS |
| Arithmetic | PASS | PASS |
| Syscalls | PASS | PASS |
| Arrays | FAIL | PASS |
| Structs | FAIL | PASS |
| Function params | FAIL | PASS |
| Function calls | PARTIAL | PASS |

## Conclusion

The MIR layer is **functionally complete for graphics demos** (loops + syscalls) but **incomplete for real programs**. The gaps are well-defined and fixable.

Priority order:
1. Parser fixes (immediate blocker)
2. VM handlers (enables execution)
3. Cleanup (code quality)

---

*Generated by Claude Code during MIR sanity check analysis.*
