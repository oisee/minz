# MIR Fixes Working Plan

**Created:** 2026-01-09
**Status:** Ready for Implementation
**Priority:** High - Blocks real program execution

## Overview

This document outlines the fixes needed to complete the MIR layer for full MinZ program support.

---

## Phase 1: Parser Alignment (Critical)

**Goal:** Parser can parse everything String() outputs

### 1.1 Array Access Syntax

**File:** `pkg/ir/mir_parser.go` (parseAssignment function)

**Add parsing for:**
```
r%d = r%d[r%d]     →  OpLoadIndex (dynamic index)
r%d = r%d[%d]      →  OpLoadElement (constant index)
r%d[r%d] = r%d     →  OpStoreIndex
r%d[%d] = r%d      →  OpStoreElement
```

**Implementation:**
```go
// In parseAssignment, before binary operators check:

// Check for array load: r0 = r1[r2] or r0 = r1[5]
if strings.Contains(expr, "[") && strings.Contains(expr, "]") {
    // Parse base[index] pattern
    bracketStart := strings.Index(expr, "[")
    bracketEnd := strings.Index(expr, "]")
    base := strings.TrimSpace(expr[:bracketStart])
    index := strings.TrimSpace(expr[bracketStart+1:bracketEnd])

    inst.Src1 = Register(p.parseRegister(base))

    if val, err := strconv.ParseInt(index, 0, 64); err == nil {
        inst.Op = OpLoadElement
        inst.Imm = val
    } else {
        inst.Op = OpLoadIndex
        inst.Src2 = Register(p.parseRegister(index))
    }
    return inst, nil
}
```

**For store (in dest parsing):**
```go
// Check for array store: r1[r2] = r3 or r1[5] = r3
if strings.Contains(dest, "[") {
    bracketStart := strings.Index(dest, "[")
    bracketEnd := strings.Index(dest, "]")
    base := strings.TrimSpace(dest[:bracketStart])
    index := strings.TrimSpace(dest[bracketStart+1:bracketEnd])

    inst.Src1 = Register(p.parseRegister(base))
    inst.Dest = Register(p.parseRegister(strings.TrimSpace(parts[1])))

    if val, err := strconv.ParseInt(index, 0, 64); err == nil {
        inst.Op = OpStoreElement
        inst.Imm = val
    } else {
        inst.Op = OpStoreIndex
        inst.Src2 = Register(p.parseRegister(index))
    }
    return inst, nil
}
```

### 1.2 Struct Field Syntax

**Add parsing for:**
```
r%d = r%d.field[%d]     →  OpLoadField
r%d.field[%d] = r%d     →  OpStoreField
```

**Implementation:**
```go
// Check for field load: r0 = r1.field[2]
if strings.Contains(expr, ".field[") {
    dotIdx := strings.Index(expr, ".field[")
    bracketEnd := strings.Index(expr, "]")
    base := strings.TrimSpace(expr[:dotIdx])
    fieldIdx := strings.TrimSpace(expr[dotIdx+7:bracketEnd])

    inst.Op = OpLoadField
    inst.Src1 = Register(p.parseRegister(base))
    inst.Imm, _ = strconv.ParseInt(fieldIdx, 0, 64)
    return inst, nil
}
```

### 1.3 Parameter Loading

**Add parsing for:**
```
r%d = param %s   →  OpLoadParam
```

**Implementation:**
```go
// Check for param load: r0 = param x
if strings.HasPrefix(expr, "param ") {
    paramName := strings.TrimPrefix(expr, "param ")
    inst.Op = OpLoadParam
    inst.Symbol = strings.TrimSpace(paramName)
    return inst, nil
}
```

### 1.4 Fix OpLoadParam Serialization

**File:** `pkg/ir/ir.go` (String function)

**Current (line ~887):**
```go
case OpLoadParam:
    return fmt.Sprintf("r%d = param %s", i.Dest, i.Symbol)
```

This is correct, but semantic analyzer may not be setting Symbol. Need to verify.

---

## Phase 2: VM Handlers

**Goal:** VM can execute all parsed opcodes

**File:** `pkg/mirvm/vm.go`

### 2.1 Array Opcodes

```go
case ir.OpLoadIndex:
    // r0 = r1[r2] - load from array at dynamic index
    baseAddr := vm.registers[inst.Src1]
    index := vm.registers[inst.Src2]
    // For now, treat as memory array
    vm.registers[inst.Dest] = vm.memory[baseAddr+index]

case ir.OpStoreIndex:
    // r1[r2] = r0 - store to array at dynamic index
    baseAddr := vm.registers[inst.Src1]
    index := vm.registers[inst.Src2]
    vm.memory[baseAddr+index] = vm.registers[inst.Dest]

case ir.OpLoadElement:
    // r0 = r1[5] - load from array at constant index
    baseAddr := vm.registers[inst.Src1]
    vm.registers[inst.Dest] = vm.memory[baseAddr+inst.Imm]

case ir.OpStoreElement:
    // r1[5] = r0 - store to array at constant index
    baseAddr := vm.registers[inst.Src1]
    vm.memory[baseAddr+inst.Imm] = vm.registers[inst.Dest]
```

### 2.2 Struct Field Opcodes

```go
case ir.OpLoadField:
    // r0 = r1.field[2] - load field at offset
    baseAddr := vm.registers[inst.Src1]
    vm.registers[inst.Dest] = vm.memory[baseAddr+inst.Imm]

case ir.OpStoreField:
    // r1.field[2] = r0 - store field at offset
    baseAddr := vm.registers[inst.Src1]
    vm.memory[baseAddr+inst.Imm] = vm.registers[inst.Src2]
```

### 2.3 Parameter Loading

```go
case ir.OpLoadParam:
    // Load function parameter - params stored in registers by convention
    // This requires tracking param->register mapping in function metadata
    paramReg := vm.findParamRegister(inst.Symbol)
    if paramReg < 0 {
        return false, fmt.Errorf("undefined parameter: %s", inst.Symbol)
    }
    vm.registers[inst.Dest] = vm.registers[paramReg]
```

### 2.4 Array Init

```go
case ir.OpArrayInit:
    // Allocate array in memory
    size := inst.Imm
    addr := vm.allocate(int(size))
    vm.registers[inst.Dest] = int64(addr)

case ir.OpArrayElement:
    // Set array element during initialization
    baseAddr := vm.registers[inst.Dest]
    vm.memory[baseAddr+inst.Imm] = inst.Imm2
```

---

## Phase 3: Test Coverage

### 3.1 Parser Tests

Add to `pkg/ir/mir_parser_test.go`:

```go
func TestParseArrayAccess(t *testing.T) {
    tests := []struct{
        input string
        op    ir.Opcode
    }{
        {"r0 = r1[r2]", ir.OpLoadIndex},
        {"r0 = r1[5]", ir.OpLoadElement},
        {"r1[r2] = r0", ir.OpStoreIndex},
        {"r1[5] = r0", ir.OpStoreElement},
    }
    // ...
}

func TestParseFieldAccess(t *testing.T) {
    tests := []struct{
        input string
        op    ir.Opcode
    }{
        {"r0 = r1.field[0]", ir.OpLoadField},
        {"r1.field[0] = r0", ir.OpStoreField},
    }
    // ...
}

func TestParseParam(t *testing.T) {
    input := "r0 = param x"
    // ...
}
```

### 3.2 E2E Tests

Add to `pkg/semantic/mir_codegen_test.go`:

```go
func TestArrayExecution(t *testing.T) {
    source := `
    fun main() -> void {
        let arr: [u8; 5];
        arr[0] = 42;
        arr[1] = arr[0] + 1;
    }
    `
    // Compile and execute via MZV
}

func TestStructExecution(t *testing.T) {
    source := `
    struct Point { x: u8, y: u8 }
    fun main() -> void {
        let p: Point;
        p.x = 10;
        p.y = p.x + 5;
    }
    `
    // Compile and execute via MZV
}

func TestFunctionWithParams(t *testing.T) {
    source := `
    fun add(a: u8, b: u8) -> u8 {
        return a + b;
    }
    fun main() -> void {
        let x: u8 = add(5, 3);
    }
    `
    // Compile and execute via MZV
}
```

---

## Phase 4: Cleanup

### 4.1 Dead Code Removal

**File:** `pkg/ir/ir.go`

Mark or remove unused opcodes:
```go
// DEPRECATED: Not used - MinZ uses monomorphization
// OpInterfaceCall
// OpMethodDispatch
// OpCastInterface
// OpCheckCast
```

### 4.2 Documentation

Update `pkg/ir/ir.go` header comment:
```go
// Opcode categories:
// - Core: Arithmetic, bitwise, comparison, control flow
// - Memory: Load/store variables, arrays, structs
// - Target-specific: SMC/TSMC (Z80 only)
// - Deprecated: Interface dispatch (unused)
```

---

## Implementation Order

| Step | Task | Files | Effort |
|------|------|-------|--------|
| 1 | Add array syntax to parser | mir_parser.go | 1 hour |
| 2 | Add field syntax to parser | mir_parser.go | 30 min |
| 3 | Add param syntax to parser | mir_parser.go | 15 min |
| 4 | Add array handlers to VM | vm.go | 1 hour |
| 5 | Add field handlers to VM | vm.go | 30 min |
| 6 | Add param handler to VM | vm.go | 30 min |
| 7 | Add parser tests | mir_parser_test.go | 1 hour |
| 8 | Add E2E tests | mir_codegen_test.go | 1 hour |
| 9 | Cleanup dead code | ir.go | 30 min |

**Total estimated effort:** ~6-7 hours

---

## Success Criteria

After implementation:

1. `go test ./pkg/ir/... ./pkg/mirvm/... ./pkg/semantic/...` - ALL PASS
2. Array test program compiles and executes correctly
3. Struct test program compiles and executes correctly
4. Function with parameters compiles and executes correctly
5. Existing gradient/loop tests still pass

---

## Notes

- Memory model: VM uses flat memory array, structs/arrays are contiguous
- Register allocation: Semantic analyzer handles this, VM just executes
- No heap yet: OpAlloc can use bump allocator in VM memory

---

*Working plan created by Claude Code.*
