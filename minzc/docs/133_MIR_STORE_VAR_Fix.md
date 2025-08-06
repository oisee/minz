# MIR STORE_VAR Symbol Fix

## Issue

The E2E backend testing revealed that `STORE_VAR` MIR instructions were missing variable names in their Symbol field, causing multiple backends to fail:

```mir
; store , r2    # Missing variable name!
; store , r4    # Should be: store x, r2
; store , r8    # Should be: store sum, r8
```

## Impact

- **Assembly backends** (Z80, 6502, etc.): Worked fine because they use memory addresses
- **C backend**: Skipped stores with empty names, producing wrong output (0 instead of 52)
- **LLVM backend**: Generated invalid IR syntax
- **WebAssembly backend**: Generated undefined global references

## Root Cause

In `pkg/semantic/analyzer.go`, the `analyzeVarDeclInFunc` function was using:
```go
irFunc.Emit(ir.OpStoreVar, reg, valueReg, 0)
```

The `Emit` method only sets `Op`, `Dest`, `Src1`, `Src2` fields but not the `Symbol` field.

## Solution

Changed to manually append instructions with all fields set:
```go
irFunc.Instructions = append(irFunc.Instructions, ir.Instruction{
    Op:     ir.OpStoreVar,
    Dest:   reg,
    Src1:   valueReg,
    Symbol: v.Name,     // Now properly sets the variable name
    Type:   varType,
})
```

## Results

After the fix:
- MIR now shows: `; store x, r2` instead of `; store , r2`
- C backend generates correct code: `x = r2;` instead of skipping
- LLVM backend generates valid syntax (though still has other issues)
- WebAssembly shows proper variable names: `global.set $x` instead of `global.set $`

## Testing

Verified with E2E backend testing:
```bash
./test_backend_e2e.sh
```

7/8 backends now pass code generation (WebAssembly still needs global declarations).

## Lessons Learned

1. Always use the full instruction construction when fields like `Symbol` are needed
2. The `Emit` helper methods are only suitable for simple instructions
3. Backend testing is crucial for catching semantic analysis bugs
4. A single bug in MIR generation can affect multiple backends differently