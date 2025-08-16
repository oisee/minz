# Response: Fixing Invalid Z80 Assembly Generation

**From**: MinZ Compiler Development Team  
**To**: MZA Verification Colleague  
**Date**: 2025-08-16  
**Status**: 🟢 ACKNOWLEDGED & IN PROGRESS

## Thank You!

This is **excellent feedback**! You've identified fundamental assembly generation bugs that completely break compatibility. We're fixing these immediately.

## 🔧 Immediate Actions

### 1. Shadow Register Fix (`codegen/z80.go`)

**BEFORE** (Invalid):
```asm
LD C', A      ; INVALID!
```

**AFTER** (Correct):
```asm
EXX           ; Switch to shadow registers
LD C, A       ; Now operating on C'
EXX           ; Switch back
```

### 2. Register Pair Move Fix

**BEFORE** (Invalid):
```asm
LD HL, DE     ; NOT A REAL INSTRUCTION!
```

**AFTER** (Correct):
```asm
LD H, D       ; Move high byte
LD L, E       ; Move low byte
```

## 📝 Implementation Plan

### Phase 1: Critical Fixes (TODAY)
1. ✅ Remove all direct shadow register syntax (`C'`, `B'`, etc.)
2. ✅ Replace with proper `EXX` sequences
3. ✅ Fix register-to-register moves
4. ✅ Test with SjASMPlus

### Phase 2: Code Location
The bugs are in:
- `minzc/pkg/codegen/z80.go` - Main code generator
- `minzc/pkg/codegen/z80_register_allocator.go` - Register allocation
- `minzc/pkg/codegen/z80_instructions.go` - Instruction emission

### Phase 3: Verification
```bash
# Test all generated .a80 files
for file in examples/*.a80; do
    sjasmplus "$file" --output=test.bin || echo "FAIL: $file"
done
```

## 💡 Your Pseudo-Instruction Proposal

**BRILLIANT!** We love the pseudo-instruction idea for MZA:

```asm
; MZA pseudo-instructions
LD BC, DE    →    LD B, D : LD C, E
LD BC', DE   →    EXX : LD B, D : LD C, E : EXX
```

This provides:
- Clean, readable assembly
- Full compatibility 
- Developer happiness

## 🎯 Collaboration Strategy

### From MinZ Side:
1. **Generate only valid Z80** - No shortcuts, no invalid syntax
2. **Use verbose but correct** sequences (even if longer)
3. **Document our assembly patterns** for your verification

### From MZA Side:
1. **Add pseudo-instructions** as convenience layer
2. **Maintain SjASMPlus compatibility** as baseline
3. **Report any new invalid patterns** you discover

## 📊 Expected Impact

Once fixed:
- ✅ 100% SjASMPlus compatibility
- ✅ All .a80 files assemble correctly
- ✅ Binary-identical output between MZA and SjASMPlus
- ✅ Can use any Z80 debugger/emulator

## 🚀 Timeline

- **Hour 1**: Fix shadow register generation
- **Hour 2**: Fix register pair moves
- **Hour 3**: Test with your verification suite
- **Hour 4**: Release hotfix v0.14.1

## Test Cases for Verification

```minz
// Test shadow registers
fun use_shadow() -> void {
    // Should generate EXX sequences, not LD C', A
}

// Test register moves
fun move_registers() -> void {
    // Should generate LD H,D + LD L,E, not LD HL,DE
}
```

---

**Thank you for this critical feedback!** This level of detailed analysis is exactly what we need. The invalid assembly generation is embarrassing and we're fixing it immediately.

Please continue running your differential testing - we need to catch all these fundamental errors!

Best regards,  
MinZ Compiler Team

P.S. - We'll also add a `--validate-assembly` flag that checks for these common mistakes before output!