# MinZ Compiler Optimization Guide: Current vs Ideal

## Detailed Comparison: simple_add Function

### Current Compiler Output (15 instructions in add, 13 in main = 28 total)

```asm
; Function: add
add:
add_param_a:
    LD HL, #0000   ; SMC parameter a        [1]
    LD ($F006), HL ; Store to memory        [2] WASTEFUL!
add_param_b:
    LD HL, #0000   ; SMC parameter b        [3]
    LD ($F008), HL ; Store to memory        [4] WASTEFUL!
    ; r5 = r3 + r4
    LD HL, ($F006) ; Load a from memory     [5] WASTEFUL!
    LD D, H        ; Transfer to DE         [6]
    LD E, L                                 [7]
    LD HL, ($F008) ; Load b from memory     [8] WASTEFUL!
    ADD HL, DE     ; Add                    [9]
    LD ($F00A), HL ; Store result           [10] WASTEFUL!
    ; return r5
    LD HL, ($F00A) ; Load result            [11] WASTEFUL!
    RET                                     [12]

; Function: main
main:
    LD A, 10       ; Load constant          [13]
    LD ($F004), A  ; Store to memory        [14] WASTEFUL!
    LD A, 20       ; Load constant          [15]
    LD ($F006), A  ; Store to memory        [16] WASTEFUL!
    CALL add       ; Call function          [17] MISSING PARAM SETUP!
    LD ($F008), HL ; Store result           [18] WASTEFUL!
    LD HL, ($F008) ; Load result            [19] WASTEFUL!
    LD ($F002), HL ; Store to x             [20] WASTEFUL!
    RET                                     [21]
```

### Ideal Code (6 instructions in add, 7 in main = 13 total)

```asm
; Function: add
add:
add_param_a:
    LD HL, #0000   ; SMC parameter a        [1]
    LD D, H        ; Save a in DE           [2] DIRECT USE!
    LD E, L                                 [3]
add_param_b:
    LD HL, #0000   ; SMC parameter b        [4]
    ADD HL, DE     ; HL = a + b             [5] DIRECT USE!
    RET                                     [6]

; Function: main
main:
    LD HL, 10                               [1]
    LD (add_param_a + 1), HL ; MODIFY SMC!  [2] KEY DIFFERENCE!
    LD HL, 20                               [3]
    LD (add_param_b + 1), HL ; MODIFY SMC!  [4] KEY DIFFERENCE!
    CALL add                                [5]
    ; Result already in HL - use directly!
    RET                                     [6]
```

## Key Differences and How to Achieve Them

### 1. **Direct Parameter Usage** (Saves 6 instructions in add)

**Current Problem:**
```asm
LD HL, #0000   ; Load parameter
LD ($F006), HL ; Store to memory (WASTEFUL!)
; ... later ...
LD HL, ($F006) ; Load back from memory (WASTEFUL!)
```

**Ideal Solution:**
```asm
LD HL, #0000   ; Load parameter
LD D, H        ; Use directly!
LD E, L
```

**How to achieve:** The semantic analyzer needs to recognize that SMC parameters are already in registers after loading. Instead of generating `StoreVar` instructions after `LoadParam`, it should keep track of which register contains which value.

### 2. **SMC Parameter Modification at Call Site** (Critical for SMC)

**Current Problem:**
```asm
; main doesn't modify SMC slots!
LD A, 10
LD ($F004), A  ; Stores to local variable
CALL add       ; But add expects modified SMC slots!
```

**Ideal Solution:**
```asm
LD HL, 10
LD (add_param_a + 1), HL  ; Modify the immediate value in add!
LD HL, 20
LD (add_param_b + 1), HL  ; Modify the immediate value in add!
CALL add
```

**How to achieve:** The semantic analyzer needs to generate different IR for function calls:
- Instead of evaluating arguments and storing them
- Generate instructions to modify the SMC parameter slots
- This requires knowing the address of each parameter slot

### 3. **Eliminate Redundant Stores** (Saves 10 instructions)

**Current Problem:**
- Every intermediate value is stored to memory
- Every value is loaded back before use
- Result is stored and reloaded unnecessarily

**Ideal Solution:**
- Keep values in registers
- Only store when absolutely necessary (e.g., before a call that might trash registers)
- Use result directly from HL after function return

**How to achieve:** Implement register allocation and liveness analysis:
- Track which values are "live" in registers
- Only spill to memory when registers are needed for something else
- Eliminate store/load pairs for temporary values

## Implementation Steps

### Step 1: Fix SMC Parameter Loading (Semantic Analyzer)
```go
// Instead of:
// LoadParam -> StoreVar sequence

// Generate:
// LoadParam with "keep in register" flag
// Track that register X contains parameter Y
```

### Step 2: Fix Function Calls (Semantic Analyzer)
```go
// For SMC functions, generate:
// 1. Calculate argument values
// 2. Generate SMC slot modifications:
//    ModifySMC target_func_param_a, value
// 3. Generate call
```

### Step 3: Register Tracking (Code Generator)
```go
type RegisterState struct {
    HL_contains Variable  // Track what's in HL
    DE_contains Variable  // Track what's in DE
    // etc.
}

// Before loading a value, check if it's already in a register
if g.regState.HL_contains == inst.Src1 {
    // Already in HL, no need to load!
}
```

### Step 4: Peephole Optimizations
Already partially implemented, but need:
- Store/Load elimination
- Dead store elimination
- SMC parameter direct use

## Expected Results

With these optimizations:
- **simple_add**: 28 instructions → 13 instructions (54% reduction)
- **Memory accesses**: 16 → 2 (87% reduction)
- **Execution time**: ~400 T-states → ~150 T-states (63% faster)

The key insight is that SMC parameters should be treated differently from regular parameters - they're not memory locations, they're embedded instructions!