# Current vs Expected SMC Generation

## The Problem

### Current (INCORRECT) Generation
```asm
; Function: simple_add.add_numbers$u8$u8
expected.simple_add.add_numbers$u8$u8:
expected.simple_add.add_numbers$u8$u8_param_a:
    LD HL, #0000   ; SMC parameter a (u8->u16)  ❌ WRONG SIZE!
expected.simple_add.add_numbers$u8$u8_param_b:
    LD DE, #0000   ; SMC parameter b (u8->u16)  ❌ WRONG SIZE!
    ; ... No EQU labels, no .op anchors
    
; At call site:
    CALL expected.simple_add.add_numbers$u8$u8  ❌ NO PATCHING!
```

### Expected (CORRECT) Generation
```asm
; Function: simple_add.add_numbers$u8$u8
simple_add.add_numbers$u8$u8:
simple_add.add_numbers$u8$u8_param_a.op:        ✅ .op anchor
simple_add.add_numbers$u8$u8_param_a equ simple_add.add_numbers$u8$u8_param_a.op + 1  ✅ EQU
    LD A, #00   ; SMC parameter a (u8)          ✅ CORRECT SIZE!
    
simple_add.add_numbers$u8$u8_param_b.op:        ✅ .op anchor
simple_add.add_numbers$u8$u8_param_b equ simple_add.add_numbers$u8$u8_param_b.op + 1  ✅ EQU
    LD B, #00   ; SMC parameter b (u8)          ✅ CORRECT SIZE!
    
; At call site:
    ld (simple_add.add_numbers$u8$u8_param_a),a  ✅ PATCH PARAM!
    ld a,b
    ld (simple_add.add_numbers$u8$u8_param_b),a  ✅ PATCH PARAM!
    CALL simple_add.add_numbers$u8$u8
```

## Key Differences

| Aspect | Current (Wrong) | Expected (Correct) |
|--------|-----------------|---------------------|
| **Instruction Size** | Always 16-bit (HL, DE) | Correct size (A for u8, HL for u16) |
| **Anchors** | Missing .op labels | Has .op: labels |
| **EQU Labels** | Missing | Points to immediate operand |
| **Parameter Passing** | Direct CALL | Patches before CALL |
| **Return Handling** | Via registers | Patches destination |
| **Stack Usage** | PUSH/POP | None (only RET address) |
| **Memory Usage** | Stores to $F000+ | None (all in code) |

## Why This Matters

### Performance Impact
- **Current**: 10+ cycles for LD HL, #0000
- **Expected**: 7 cycles for LD A, #00
- **Savings**: 30% faster parameter loading

### Memory Impact
- **Current**: Uses RAM at $F000+ for variables
- **Expected**: Zero RAM usage, all in code

### Code Size Impact
- **Current**: 3 bytes for LD HL, #0000
- **Expected**: 2 bytes for LD A, #00
- **Savings**: 33% smaller per parameter

## The Philosophy

TRUE SMC is about making the **code modify itself** instead of using traditional parameter passing:

1. **Parameters are immediates** - Not in registers or stack
2. **Calls are patches** - Modify the target function before calling
3. **Returns are stores** - Function patches where to put result
4. **Code is the data** - No separation between program and data

## Implementation Priority

1. **CRITICAL**: Fix instruction sizes (LD A instead of LD HL for u8)
2. **HIGH**: Add .op anchors and EQU labels
3. **HIGH**: Implement parameter patching at call sites
4. **MEDIUM**: Implement return address patching
5. **LOW**: Optimize patch sequences

## Validation

To verify correct implementation:

```bash
# Should see .op labels
grep "\.op:" generated.a80

# Should see EQU definitions
grep "equ.*\.op + 1" generated.a80

# Should see parameter patching
grep "ld ([a-z_]*param[a-z_]*)" generated.a80

# Should NOT see these for u8 params:
grep "LD HL, #0000.*u8" generated.a80  # This is wrong!
```