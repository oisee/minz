# TRUE SMC Implementation Roadmap

## Current State vs Target State

### Current Implementation (AS-IS)
```asm
; Current parameter passing (WRONG)
func_param_a:
    LD HL, #0000   ; Using 16-bit for 8-bit value
    ; No EQU labels, no .op anchors
```

### Target Implementation (TO-BE)
```asm
; TRUE SMC parameter passing (CORRECT)
func_param_a.op:
func_param_a equ func_param_a.op + 1
    LD A, #00      ; Correct size, proper anchors
```

## Implementation Steps

### Step 1: Fix Instruction Generation (URGENT)
**File:** `minzc/pkg/codegen/z80.go`

#### 1.1 Generate Proper Anchors
```go
// Instead of:
g.emit("%s_param_%s:", funcName, paramName)
g.emit("    LD HL, #0000")

// Generate:
g.emit("%s_param_%s.op:", funcName, paramName)
g.emit("%s_param_%s equ %s_param_%s.op + 1", funcName, paramName, funcName, paramName)
g.emit("    LD A, #00   ; SMC parameter %s (%s)", paramName, paramType)
```

#### 1.2 Use Correct Register Sizes
- u8 → Use A, B, C, D, E, H, L registers
- u16 → Use BC, DE, HL registers
- bool → Use A register (0 or 1)
- Pointers → Use HL register

### Step 2: Implement Call-Site Patching
**File:** `minzc/pkg/codegen/z80.go` - `generateCall` function

```go
// Before CALL, generate patch sequence:
for _, param := range callParams {
    if param.Type.Size() == 1 {
        // 8-bit patch
        g.emit("    LD A, %s", param.Source)
        g.emit("    LD (%s_param_%s), A", targetFunc, param.Name)
    } else {
        // 16-bit patch
        g.emit("    LD HL, %s", param.Source)
        g.emit("    LD (%s_param_%s), HL", targetFunc, param.Name)
    }
}
```

### Step 3: Implement Return Address Patching
**File:** `minzc/pkg/codegen/z80.go`

```go
// Generate return anchor in function:
g.emit("%s_return.op:", funcName)
g.emit("%s_return equ %s_return.op + 1", funcName, funcName)
if returnType.Size() == 1 {
    g.emit("    LD (#0000), A   ; Return value destination")
} else {
    g.emit("    LD (#0000), HL  ; Return value destination")
}

// At call site, patch return destination:
g.emit("    LD HL, %s", resultLocation)
g.emit("    LD (%s_return), HL", targetFunc)
```

### Step 4: Handle Local Variables as SMC
**File:** `minzc/pkg/semantic/codegen.go`

```go
// For each local variable, generate anchor:
g.emit("%s_%s.op:", funcName, varName)
g.emit("%s_%s equ %s_%s.op + 1", funcName, varName, funcName, varName)
g.emit("    LD A, #%02X   ; %s = %d", value, varName, value)
```

## Testing Matrix

| Test Case | File | Status | Description |
|-----------|------|--------|-------------|
| 8-bit params | `expected/simple_add.a80` | ✅ Reference | Basic 8-bit addition |
| 16-bit params | `expected/test_16bit_params.a80` | ✅ Created | 16-bit parameter passing |
| Mixed params | `expected/test_mixed_params.a80` | ✅ Created | Mix of 8/16/bool params |
| Multi-return | `expected/test_multi_return.a80` | ✅ Created | Multiple return values |
| Recursion | `expected/test_recursive_smc.a80` | ❌ TODO | Recursive SMC |
| Structs | `expected/test_struct_smc.a80` | ❌ TODO | Struct parameters |
| Arrays | `expected/test_array_smc.a80` | ❌ TODO | Array access |

## Code Changes Required

### 1. `z80.go::generateTrueSMCFunction`
- [ ] Generate .op labels for all anchors
- [ ] Generate EQU definitions
- [ ] Use correct register sizes
- [ ] Remove 16-bit extension for 8-bit values

### 2. `z80.go::generateCall`
- [ ] Detect SMC functions
- [ ] Generate parameter patch sequence
- [ ] Generate return address patch
- [ ] Call the function

### 3. `z80.go::generateParameterAnchor`
- [ ] Create new function for anchor generation
- [ ] Handle different parameter types
- [ ] Generate correct instruction for type

### 4. `ir.go::Function`
- [ ] Add `UsesTrueSMC` flag
- [ ] Track parameter anchors
- [ ] Track return anchors

## Validation Criteria

### Assembly Pattern Validation
```bash
# Check for proper anchors
grep -E "\.op:" generated.a80
grep -E "equ.*\.op \+ 1" generated.a80

# Check for patch instructions
grep -E "LD \([a-z_]+_param_" generated.a80
grep -E "LD \([a-z_]+_return\)" generated.a80

# Check instruction sizes
grep -E "LD [ABCDEHL], #" generated.a80  # Should see 8-bit
grep -E "LD (BC|DE|HL), #" generated.a80 # Should see 16-bit
```

### Functional Validation
1. Compile test programs
2. Assemble with sjasmplus
3. Run in emulator
4. Verify correct results

## Performance Metrics

### Current Implementation
- Stack usage: ~4-8 bytes per call
- Memory usage: Variables in RAM
- Cycles: PUSH/POP overhead

### TRUE SMC Implementation
- Stack usage: 0 bytes (only return address)
- Memory usage: 0 bytes (all in code)
- Cycles: Direct immediate operations

## Timeline

| Phase | Task | Estimate | Priority |
|-------|------|----------|----------|
| 1 | Fix instruction generation | 2 hours | CRITICAL |
| 2 | Implement call-site patching | 3 hours | HIGH |
| 3 | Implement return patching | 2 hours | HIGH |
| 4 | Handle local variables | 2 hours | MEDIUM |
| 5 | Test and validate | 3 hours | HIGH |
| 6 | Optimize patch sequences | 2 hours | LOW |

## Success Criteria

✅ **DONE** when:
1. Generated assembly matches `expected/*.a80` patterns
2. All test cases pass
3. No stack usage for parameters
4. All values live in instruction immediates
5. Function calls perform patching

## References

- Philosophy: `docs/142_TRUE_SMC_Philosophy_Complete.md`
- Original Design: `docs/018_TRUE_SMC_Design_v2.md`
- Expected Examples: `expected/` directory
- Current Implementation: `minzc/pkg/codegen/z80.go`