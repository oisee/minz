# Local Variable Addressing Fix
**Date**: July 26, 2025  
**Issue**: Critical bug where all local variables used same memory address  
**Status**: FIXED ✓

## Problem

All local variables in functions were being allocated to the same memory address ($F000), causing data corruption.

### Before Fix
```asm
; r5 = load a     
LD HL, ($F000)    ; ❌ Wrong address
; r6 = load b
LD HL, ($F000)    ; ❌ Same address!
; r15 = load c
LD HL, ($F000)    ; ❌ Same address!
```

### After Fix
```asm
; r5 = load a     
LD HL, ($F002)    ; ✓ Unique address
; r6 = load b
LD HL, ($F006)    ; ✓ Different address
; r15 = load c
LD HL, ($F016)    ; ✓ Different address
```

## Root Cause

The issue had multiple layers:

1. **Missing Register Mapping**: OpLoadVar had symbol name but no register reference
2. **No Address Allocation**: Local variables weren't assigned memory addresses
3. **Fallback to Zero**: Untracked locals defaulted to register 0 → $F000

## Solution Implemented

### 1. Address Allocation in Code Generator
```go
// In generateFunction()
localOffset := uint16(0)
for _, local := range fn.Locals {
    addr := g.localVarBase + localOffset
    g.regAlloc.SetAddress(local.Reg, addr)
    localOffset += uint16(local.Type.Size())
}
```

### 2. Register Allocator Enhancement
```go
// Added to RegisterAllocator
addresses map[ir.Register]uint16

func (r *RegisterAllocator) SetAddress(reg ir.Register, addr uint16) {
    r.addresses[reg] = addr
}

func (r *RegisterAllocator) GetAddress(reg ir.Register) (uint16, bool) {
    addr, ok := r.addresses[reg]
    return addr, ok
}
```

### 3. Variable Lookup by Name
```go
// In OpLoadVar handler
localReg := ir.Register(0)
for _, local := range g.currentFunc.Locals {
    if local.Name == inst.Symbol {
        localReg = local.Reg
        break
    }
}
addr := g.getAbsoluteAddr(localReg)
```

## Testing

### Test Program
```minz
fun test_locals() {
    let u16 a = 0x1234;
    let u16 b = 0x5678;
    let u16 c = 100;
    
    g_sum = a + b;    // Should be 0x68AC
    g_product = b * c; // Should be 22136
}
```

### Verification
```bash
# Check generated assembly
grep "LD HL, (\$F0" test.a80 | sort | uniq -c
# Should show different addresses, not all $F000
```

## Impact

This fix enables:
- Multiple local variables in functions
- Complex calculations with temporaries
- Proper function parameter handling
- Reliable data flow in programs

## Lessons Learned

1. **Complete the Chain**: Ensure data flows through entire compilation pipeline
2. **Test Multi-Variable**: Always test with multiple variables
3. **Defensive Defaults**: Default behaviors should fail loudly, not silently
4. **Address Tracking**: Memory allocation must be explicit and tracked

## Future Improvements

1. **Stack-Based Locals**: Use IX+offset for true stack allocation
2. **Register Variables**: Keep frequently used vars in registers
3. **Live Range Analysis**: Reuse addresses for non-overlapping variables
4. **Debug Info**: Emit variable-to-address mapping for debuggers