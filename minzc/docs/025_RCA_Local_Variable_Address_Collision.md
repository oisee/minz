# Root Cause Analysis: Local Variable Address Collision
**Date**: July 26, 2025  
**Severity**: CRITICAL 🔴  
**Component**: Code Generator / Semantic Analyzer

## Problem Statement

All local variables in MinZ functions are assigned the same memory address ($F000), causing data corruption and incorrect program behavior.

## Symptoms

```asm
; Expected: Different addresses for each local
r5 = load a     ; LD HL, ($F002)  
r6 = load b     ; LD HL, ($F006)
r15 = load c    ; LD HL, ($F016)

; Actual: All use same address
r5 = load a     ; LD HL, ($F000)  
r6 = load b     ; LD HL, ($F000)  <-- BUG!
r15 = load c    ; LD HL, ($F000)  <-- BUG!
```

## Root Cause Chain

### 1. Local Variable Declaration (Semantic Analysis)
```go
// analyzeVarDeclInFunc() in semantic/analyzer.go
reg := irFunc.AddLocal(v.Name, varType)
// ✓ Correctly adds local to IR function
// ✗ Does NOT assign memory address
```

### 2. Variable Load Generation (Semantic Analysis)
```go
// analyzeIdentifier() 
case *VarSymbol:
    if sym.Register != 0 {
        irFunc.EmitLoadVar(resultReg, sym.Register, id.Name)
    }
// ✗ Local variables have Register = 0
```

### 3. Address Resolution (Code Generation)
```go
// getLocalAddr() in codegen/z80.go
for _, local := range g.currentFunc.Locals {
    if local.Name == name {
        return g.getRegAddr(local.Reg)
    }
}
return g.getRegAddr(0)  // ← DEFAULT TO REGISTER 0!
```

### 4. Register Address Mapping
```go
// getRegAddr() 
func (g *Z80Generator) getRegAddr(reg ir.Register) uint16 {
    return g.regAlloc.GetAddress(reg)
}
// Register 0 → $F000 (always)
```

## Why It Happens

1. **Missing Address Assignment**: Local variables are tracked in IR but never assigned unique addresses
2. **Fallback Behavior**: When local not found in tracking, defaults to register 0
3. **Register 0 Collision**: All untracked locals map to $F000

## Data Flow

```
VarDecl → AddLocal(name, type) → Local{Name: "a", Reg: r1}
                                          ↓
                                  (No address assigned)
                                          ↓
LoadVar → getLocalAddr("a") → Not in tracking → Return reg 0 → $F000
```

## Impact Analysis

### Immediate Impact
- **Data Corruption**: Variables overwrite each other
- **Logic Errors**: Comparisons use wrong values  
- **Arithmetic Errors**: Operations on corrupted data

### Example Failure
```minz
fun test() {
    let a = 10;    // Stored at $F000
    let b = 20;    // Overwrites $F000
    let c = a + b; // Loads 20 + 20 = 40 (wrong!)
}
```

## Solution Design

### Option 1: Allocate During Semantic Analysis (Recommended)
```go
// In analyzeVarDeclInFunc()
reg := irFunc.AddLocal(v.Name, varType)
addr := a.allocateLocalAddress(varType.Size())
a.localAddresses[v.Name] = addr
```

### Option 2: Allocate During Code Generation
```go
// In generateFunction()
offset := 0
for _, local := range fn.Locals {
    g.localAddresses[local.Name] = g.localVarBase + offset
    offset += local.Type.Size()
}
```

### Option 3: Use Stack-Relative Addressing
```go
// Allocate locals on stack with IX+offset
// More complex but more flexible
```

## Implementation Plan

### Step 1: Track Local Addresses
```go
type Analyzer struct {
    // ... existing fields
    localAddresses map[string]uint16  // NEW
    nextLocalAddr  uint16             // NEW
}
```

### Step 2: Allocate on Declaration
```go
func (a *Analyzer) analyzeVarDeclInFunc(v *ast.VarDecl, irFunc *ir.Function) error {
    // ... existing type checking
    
    reg := irFunc.AddLocal(v.Name, varType)
    
    // NEW: Allocate address
    addr := a.nextLocalAddr
    a.localAddresses[v.Name] = addr
    a.nextLocalAddr += varType.Size()
    
    // Store in symbol
    varSym.Address = addr
}
```

### Step 3: Use Correct Address
```go
func (g *Z80Generator) getLocalAddr(name string) uint16 {
    // Check function locals first
    for _, local := range g.currentFunc.Locals {
        if local.Name == name {
            if addr, ok := g.localAddresses[name]; ok {
                return addr  // Use tracked address
            }
        }
    }
    // ... existing fallback
}
```

## Testing Strategy

### Unit Test
```go
func TestLocalVariableAddresses(t *testing.T) {
    code := `
    fun test() {
        let u8 a = 10;
        let u16 b = 20;
        let u8 c = 30;
    }
    `
    // Verify: a@$F000, b@$F001, c@$F003
}
```

### Integration Test
```minz
fun verify_locals() -> bool {
    let a = 10;
    let b = 20;
    let c = 30;
    
    return (a == 10) && (b == 20) && (c == 30);
}
```

### Regression Prevention
1. Add address tracking to IR dump
2. Verify unique addresses in MIR output
3. Add assertion in code generator

## Timeline

1. **Immediate Fix** (1 hour): Implement Option 2 in code generator
2. **Proper Fix** (2 hours): Implement Option 1 with semantic tracking  
3. **Testing** (1 hour): Unit + integration tests
4. **Documentation** (30 min): Update compiler architecture docs

## Lessons Learned

1. **Incomplete Implementation**: Feature (locals) partially implemented
2. **Silent Failure**: No error on address collision
3. **Missing Tests**: No tests for multi-variable functions
4. **Default Behavior**: Dangerous fallback to register 0

## Prevention

1. **Design Review**: Ensure complete implementation path
2. **Assertions**: Add address uniqueness checks
3. **Comprehensive Tests**: Test multi-variable scenarios
4. **No Silent Defaults**: Error on missing addresses