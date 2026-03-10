# Week 1: Quick Wins Sprint

**Цель:** MIR layer полностью работает + DX improvements
**Effort:** ~8 часов
**Expected Result:** +15% compilation success, VM executes real programs

---

## Day 1: MIR Parser Fixes (3 часа)

### Task 1: Array Access Syntax (1 час)
**File:** `minzc/pkg/ir/mir_parser.go`

```go
// Добавить в parseAssignment():

// r0 = r1[r2] - dynamic index
// r0 = r1[5]  - constant index
if strings.Contains(expr, "[") && strings.Contains(expr, "]") {
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

**Test:**
```bash
echo 'r0 = r1[r2]' | go test -run TestParseArrayAccess
```

---

### Task 2: Struct Field Syntax (30 мин)
**File:** `minzc/pkg/ir/mir_parser.go`

```go
// r0 = r1.field[2] - load field at offset
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

---

### Task 3: Parameter Loading (15 мин)
**File:** `minzc/pkg/ir/mir_parser.go`

```go
// r0 = param x
if strings.HasPrefix(expr, "param ") {
    paramName := strings.TrimPrefix(expr, "param ")
    inst.Op = OpLoadParam
    inst.Symbol = strings.TrimSpace(paramName)
    return inst, nil
}
```

---

## Day 2: VM Handlers (2 часа)

### Task 4: Array Handlers
**File:** `minzc/pkg/mirvm/vm.go`

```go
case ir.OpLoadIndex:
    baseAddr := vm.registers[inst.Src1]
    index := vm.registers[inst.Src2]
    vm.registers[inst.Dest] = vm.memory[baseAddr+index]

case ir.OpStoreIndex:
    baseAddr := vm.registers[inst.Src1]
    index := vm.registers[inst.Src2]
    vm.memory[baseAddr+index] = vm.registers[inst.Dest]

case ir.OpLoadElement:
    baseAddr := vm.registers[inst.Src1]
    vm.registers[inst.Dest] = vm.memory[baseAddr+inst.Imm]

case ir.OpStoreElement:
    baseAddr := vm.registers[inst.Src1]
    vm.memory[baseAddr+inst.Imm] = vm.registers[inst.Dest]
```

### Task 5: Field Handlers
```go
case ir.OpLoadField:
    baseAddr := vm.registers[inst.Src1]
    vm.registers[inst.Dest] = vm.memory[baseAddr+inst.Imm]

case ir.OpStoreField:
    baseAddr := vm.registers[inst.Src1]
    vm.memory[baseAddr+inst.Imm] = vm.registers[inst.Src2]
```

### Task 6: Param Handler
```go
case ir.OpLoadParam:
    paramReg := vm.findParamRegister(inst.Symbol)
    if paramReg < 0 {
        return false, fmt.Errorf("undefined parameter: %s", inst.Symbol)
    }
    vm.registers[inst.Dest] = vm.registers[paramReg]
```

---

## Day 3: Error Messages (3 часа)

### Task 7: Add Line Numbers to Errors
**File:** `minzc/pkg/semantic/analyzer.go`

**Current:**
```
error: unknown type 'foo'
```

**Target:**
```
error: examples/test.minz:15:10: unknown type 'foo'
```

**Implementation:**
1. Track source position in AST nodes
2. Pass position to error reporting
3. Format errors with `file:line:col:` prefix

---

## Day 4-5: Testing & Polish

### Task 8: Parser Tests
**File:** `minzc/pkg/ir/mir_parser_test.go`

```go
func TestParseArrayAccess(t *testing.T) {
    tests := []struct{
        input string
        op    ir.Opcode
    }{
        {"r0 = r1[r2]", ir.OpLoadIndex},
        {"r0 = r1[5]", ir.OpLoadElement},
    }
    for _, tt := range tests {
        inst, err := parser.ParseInstruction(tt.input)
        assert.NoError(t, err)
        assert.Equal(t, tt.op, inst.Op)
    }
}
```

### Task 9: E2E Tests
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
```

### Task 10: Documentation Sync
Update README.md:
- Pattern matching: "syntax works, codegen partial"
- Generics: "parked, use overloading"
- Nested functions: "not working"

---

## Verification Checklist

- [ ] `go test ./minzc/pkg/ir/...` — ALL PASS
- [ ] `go test ./minzc/pkg/mirvm/...` — ALL PASS
- [ ] Array test program executes correctly
- [ ] Struct test program executes correctly
- [ ] Function with params executes correctly
- [ ] Error messages show line numbers
- [ ] README reflects actual status

---

## Success Metrics

| Metric | Before | After |
|--------|--------|-------|
| MIR Parser coverage | ~80% | 100% |
| VM opcode coverage | ~85% | 100% |
| Error message quality | Poor | Good |
| Docs accuracy | ~65% | ~85% |

---

**Next:** Week 2 — Nested Functions + Parser Research
