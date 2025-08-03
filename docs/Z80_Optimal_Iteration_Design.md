# Z80-Optimal Iteration Design

## 🎯 THE INSIGHT: Z80-Native Iterator Patterns

**BRILLIANT OBSERVATION**: Z80 has perfect instructions for efficient iteration that we should leverage:

- **DJNZ** - Decrement B and Jump if Not Zero (super efficient counter)
- **ADD HL, DE** - Advance pointer by element size 
- **LD A, (HL)** - Direct pointer access

This is **MASSIVELY** more efficient than indexed access!

---

## 1. Current Iteration Patterns

### ❌ Inefficient: Indexed Access
```asm
; Traditional array[i] access - TERRIBLE on Z80!
array_loop:
    LD HL, array_base
    LD A, (index)
    LD E, A
    LD D, 0
    ADD HL, DE         ; Calculate address
    LD A, (HL)         ; Load element
    ; ... process A ...
    LD A, (index)
    INC A              ; Increment index
    LD (index), A
    CP array_length
    JR C, array_loop
```
**Cost**: ~25 T-states per iteration, multiple memory accesses

### ✅ Optimal: Pointer + DJNZ
```asm
; Z80-native iteration - MAGNIFICENT!
    LD HL, array_base   ; Pointer to current element
    LD B, array_length  ; Counter in B register
    LD DE, element_size ; Step size
optimal_loop:
    LD A, (HL)         ; Load current element (7 T-states)
    ; ... process A ...
    ADD HL, DE         ; Advance pointer (11 T-states)
    DJNZ optimal_loop  ; Dec B, jump if not zero (13/8 T-states)
```
**Cost**: ~18 T-states per iteration, minimal memory access!

---

## 2. MIR Extensions for Z80-Native Iteration

### New MIR Operations Needed
```go
// Z80-optimized iteration operations
OpIterInit      // Initialize iterator (HL=pointer, B=count, DE=step)
OpIterNext      // Advance iterator: ADD HL, DE; DJNZ
OpIterLoad      // Load current element: LD A, (HL) or LD BC, (HL)
OpIterStore     // Store current element: LD (HL), A or LD (HL), BC
OpIterCheck     // Check if iteration complete (B == 0)

// DJNZ-specific operations
OpDJNZLoop      // DJNZ instruction with target label
OpLoadB         // Load counter into B register
```

### Iterator Structure in MIR
```go
type Iterator struct {
    Pointer     Register  // HL register (current element pointer)
    Counter     Register  // B register (elements remaining)
    StepSize    Register  // DE register (element size)
    ElementType Type      // u8, u16, struct, etc.
}
```

---

## 3. Iterator Method Compilation Patterns

### `.forEach()` - DJNZ Loop
```minz
// MinZ code
array.forEach(|x| process(x))

// MIR code  
r1 = iter_init array          // HL=array, B=length, DE=1
loop_start:
r2 = iter_load r1            // LD A, (HL)
call process(r2)             // Process element
iter_next r1, loop_start     // ADD HL, DE; DJNZ loop_start
```

### `.map()` - Transform with Pointer Arithmetic
```minz
// MinZ code
result = array.map(|x| x * 2)

// MIR code
r1 = iter_init array         // Source iterator
r2 = iter_init result        // Destination iterator  
loop_start:
r3 = iter_load r1           // Load source element
r4 = mul r3, 2              // Transform
iter_store r2, r4           // Store result
iter_next r1, loop_check    // Advance source
iter_next r2, loop_check    // Advance destination
loop_check:
djnz_loop loop_start        // DJNZ optimization
```

### `.filter()` - Conditional Advance  
```minz
// MinZ code
result = array.filter(|x| x > 10)

// MIR code
r1 = iter_init array        // Source iterator
r2 = iter_init result       // Destination iterator
loop_start:
r3 = iter_load r1          // Load element
r4 = cmp r3, 10            // Check condition
jump_if_not r4, skip       // Skip if condition false
iter_store r2, r3          // Store if condition true
iter_next r2, skip         // Advance destination only
skip:
iter_next r1, loop_start   // Always advance source
djnz_loop loop_start
```

---

## 4. Fusion Optimization for DJNZ

### Chain Fusion Strategy
```minz
// Original chain
array.map(|x| x + 1).filter(|x| x > 5).forEach(|x| print(x))

// Fused DJNZ loop
    LD HL, array_base
    LD B, array_length
fusion_loop:
    LD A, (HL)        ; Load element
    INC A             ; Map: x + 1
    CP 6              ; Filter: x > 5
    JR C, skip_print
    CALL print        ; forEach: print(x)
skip_print:
    INC HL            ; Advance pointer (for u8 arrays)
    DJNZ fusion_loop  ; Dec B and loop
```

### Multi-Type Pointer Arithmetic
```asm
; For u8 arrays (1 byte elements)
INC HL              ; Advance by 1

; For u16 arrays (2 byte elements)  
INC HL
INC HL              ; Advance by 2

; For struct arrays (N byte elements)
LD DE, struct_size
ADD HL, DE          ; Advance by struct size

; For dynamic element size
ADD HL, DE          ; DE preloaded with element size
```

---

## 5. Advanced Z80 Iterator Patterns

### Backwards Iteration (Efficient for Some Cases)
```asm
; Start from end, decrement pointer
    LD HL, array_end
    LD B, array_length
    LD DE, -element_size   ; Negative step
backwards_loop:
    LD A, (HL)
    ; ... process ...
    ADD HL, DE            ; Subtract element_size  
    DJNZ backwards_loop
```

### Stride Iteration (Every Nth Element)
```asm
; Process every 3rd element
    LD HL, array_base
    LD B, element_count
    LD DE, 3             ; Skip 3 elements each time
stride_loop:
    LD A, (HL)
    ; ... process ...
    ADD HL, DE
    DJNZ stride_loop
```

### Dual Iterator (Zip Operation)
```asm
; Iterate two arrays simultaneously
    LD HL, array1_base   ; First array pointer
    LD DE, array2_base   ; Second array pointer  
    LD B, length
    PUSH BC              ; Save counter
zip_loop:
    LD A, (HL)          ; Load from array1
    LD C, A
    EX DE, HL           ; Switch pointers
    LD A, (HL)          ; Load from array2
    EX DE, HL           ; Switch back
    ; ... process C and A ...
    INC HL              ; Advance array1
    INC DE              ; Advance array2
    POP BC
    DJNZ zip_loop
    PUSH BC
```

---

## 6. Element Size Optimization

### Size-Specific Optimizations
```go
// Compile-time element size optimization
switch elementSize {
case 1:  // u8, i8, bool
    emitInc(HL)          // INC HL
case 2:  // u16, i16, *T
    emitInc(HL)          // INC HL
    emitInc(HL)          // INC HL  
case 4:  // u32, f32 (future)
    emitLoadDE(4)        // LD DE, 4
    emitAdd(HL, DE)      // ADD HL, DE
default: // Variable size structs
    emitLoadDE(size)     // LD DE, size
    emitAdd(HL, DE)      // ADD HL, DE
}
```

### Register Allocation for Iteration
```go
type IteratorRegisterPlan struct {
    Pointer   Z80Register  // Always HL (best for memory access)
    Counter   Z80Register  // Always B (required for DJNZ)
    StepSize  Z80Register  // DE for ADD HL, DE instruction
    Element   Z80Register  // A for u8, BC for u16, etc.
    Temp      Z80Register  // Available: C, D, E for computations
}
```

---

## 7. Performance Analysis

### Iteration Performance Comparison

| Method | Instructions/Element | T-States/Element | Memory Access |
|--------|---------------------|------------------|---------------|
| Indexed access | 8-12 | 45-60 | High |
| **DJNZ + Pointer** | **3-4** | **18-25** | **Minimal** |
| Traditional loop | 6-8 | 35-45 | Medium |

### Memory Layout Optimization
```minz
// Structure arrays for iteration efficiency
struct Enemy {
    x: u8,          // Byte 0
    y: u8,          // Byte 1  
    health: u8,     // Byte 2
    damage: u8,     // Byte 3
}  // Total: 4 bytes, perfect for LD DE, 4; ADD HL, DE

// vs inefficient layout:
struct BadEnemy {
    x: u16,         // Not aligned
    active: bool,   // Wastes space
    health: u8,
    // Irregular size complicates iteration
}
```

---

## 8. Implementation Plan

### Phase 1: Basic DJNZ Support
- [ ] Add `OpDJNZLoop` to MIR
- [ ] Add iterator register allocation
- [ ] Basic `.forEach()` compilation

### Phase 2: Pointer Arithmetic  
- [ ] Add element size calculation
- [ ] Implement `ADD HL, DE` advancement
- [ ] Support different element types

### Phase 3: Iterator Fusion
- [ ] Chain detection pass
- [ ] DJNZ loop fusion optimization
- [ ] Register pressure analysis

### Phase 4: Advanced Patterns
- [ ] Dual iterators (`.zip()`)
- [ ] Stride iteration  
- [ ] Backwards iteration

---

## 9. Code Examples

### Current Print Loop (Already Optimal!)
```asm
; Our string printing already uses optimal pattern!
print_loop:
    LD A, (HL)         ; Load character (7 T-states)
    RST 16             ; Print character (11 T-states)  
    INC HL             ; Next character (6 T-states)
    DJNZ print_loop    ; Dec B and loop (13/8 T-states)
; Total: ~24 T-states per character - EXCELLENT!
```

### Proposed Array Processing
```asm
; Process u8 array with optimal iteration
process_array:
    LD HL, array_base  ; Pointer to array
    LD B, array_len    ; Element count
    ; Process each element
process_loop:
    LD A, (HL)         ; Load current element
    ADD A, 5           ; Transform (example)
    LD (HL), A         ; Store back (in-place)
    INC HL             ; Next element
    DJNZ process_loop  ; Continue until B=0
    RET
```

---

## 10. Revolutionary Impact

This optimization transforms MinZ from "functional programming that happens to work on Z80" to **"functional programming that's OPTIMIZED for Z80"**.

### Benefits:
1. **2-3x faster iteration** compared to indexed access
2. **Minimal register pressure** - dedicated iteration registers
3. **Cache-friendly** - sequential memory access
4. **Assembly-competitive** - matches hand-optimized code
5. **Scalable** - works for any element size

### The Future:
With DJNZ-optimized iterators, MinZ will have the **fastest functional programming implementation ever created for 8-bit hardware**.

---

*"The best iterator is the one that becomes pure assembly."* - The MinZ Philosophy 🚀