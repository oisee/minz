# MinZ ABI and Calling Conventions

**Date**: July 27, 2025  
**Version**: v0.4.0-alpha  
**Purpose**: Document all supported and planned function calling conventions

## What is ABI?

**ABI (Application Binary Interface)** defines how functions communicate:
- How parameters are passed (registers, stack, memory)
- How return values are handled
- Which registers must be preserved
- Stack frame layout
- Alignment requirements

## MinZ Calling Convention Philosophy

MinZ uses **adaptive ABI selection** - the compiler analyzes each function and chooses the optimal calling convention. Functions can also be annotated to force specific conventions.

```minz
// Compiler chooses optimal ABI
fun add(a: u8, b: u8) -> u8 { return a + b; }

// Force specific ABI via annotation
@abi("fastcall")
fun fast_add(a: u8, b: u8) -> u8 { return a + b; }
```

## Supported Calling Conventions

### 1. **True SMC (Self-Modifying Code)** - RAM-only
**Status**: ✅ Implemented  
**Use case**: RAM-based code, maximum flexibility

```asm
; Caller patches the function directly
LD HL, #42
LD (add_param_a), HL    ; Modify instruction

; Function with embedded parameters
add:
add_param_a:
    LD HL, #0000        ; This value is patched
add_param_b:
    LD DE, #0000        ; This value is patched
    ADD HL, DE
    RET
```

**Pros**: 
- No stack overhead
- Parameters become immediate values
- Very fast for RAM execution

**Cons**:
- Cannot run from ROM
- Not thread-safe
- Limited to small parameters

### 2. **Virtual Register** - Memory-based
**Status**: ✅ Implemented (current default)  
**Use case**: Simple, portable

```asm
; Parameters in fixed memory locations
LD HL, #42
LD ($F000), HL          ; param 1
LD HL, #24  
LD ($F002), HL          ; param 2
CALL add

add:
    LD HL, ($F000)      ; Load param 1
    LD DE, ($F002)      ; Load param 2
    ADD HL, DE
    RET
```

**Pros**:
- Simple to implement
- Works from ROM
- Unlimited parameters

**Cons**:
- Slow memory access
- Not reentrant
- Memory contention

### 3. **Physical Register** - Z80 registers
**Status**: 🚧 Partially implemented  
**Use case**: Maximum performance

```asm
; Parameters in registers (like fastcall)
LD A, 42                ; param 1 in A
LD E, 24                ; param 2 in E
CALL add

add:
    ADD A, E            ; Direct register operation
    RET                 ; Result in A
```

**Register allocation strategies**:
```
8-bit params:  A, E, D, C, B, L, H
16-bit params: HL, DE, BC
Mixed:         HL + A, DE + C, etc.
```

**Pros**:
- Fastest possible
- No memory access
- Natural for Z80

**Cons**:
- Limited registers
- Complex allocation
- Register pressure

### 4. **Shadow Register** - Alternative set
**Status**: 📋 Planned  
**Use case**: Interrupt handlers, parallel processing

```asm
; Use shadow registers for parameters
LD A, 42
EX AF, AF'              ; Move to shadow A
LD HL, 1000
EXX                     ; Move to shadow HL
CALL shadow_func

shadow_func:
    EX AF, AF'          ; Access shadow A
    EXX                 ; Access shadow HL
    ; Process with shadow registers
    ; No need to save main registers!
    EX AF, AF'
    EXX
    RET
```

**Pros**:
- Double register capacity
- Fast context switch
- Great for interrupts

**Cons**:
- EXX overhead (4 T-states)
- Complex state management
- Not all Z80s have shadows

### 5. **Stack Frame (IX-based)** - Traditional
**Status**: 📋 Planned for v0.4.0  
**Use case**: Recursive functions, C compatibility

```asm
; Traditional stack-based calling
LD HL, 42
PUSH HL                 ; Push param 2
LD HL, 24
PUSH HL                 ; Push param 1  
CALL add
POP HL                  ; Clean stack
POP HL

add:
    PUSH IX
    LD IX, SP
    ; IX+4 = param 1
    ; IX+6 = param 2
    LD L, (IX+4)
    LD H, (IX+5)
    LD E, (IX+6)
    LD D, (IX+7)
    ADD HL, DE
    POP IX
    RET
```

**Pros**:
- Fully reentrant
- Supports recursion
- Standard approach
- Unlimited parameters

**Cons**:
- Slow stack access
- IX register tied up
- Stack overhead

### 6. **Parallel Stack Frame (IY-based)** - Dual stack
**Status**: 🔬 Experimental idea  
**Use case**: Coroutines, dual contexts

```asm
; Two parallel stacks - main (SP) and alternate (IY)
; Main context uses IX
; Alternate context uses IY

; Can run two functions "in parallel"
main_func:              ; Uses IX for locals
    PUSH IX
    LD IX, SP
    ; Access locals via IX
    
alt_func:               ; Uses IY for locals  
    PUSH IY
    LD IY, ALT_SP       ; Alternate stack
    ; Access locals via IY
```

**Pros**:
- Two contexts simultaneously
- Coroutine support
- Novel approach

**Cons**:
- Complex management
- IY often used by system
- Experimental

### 7. **Hybrid Register+Stack** - Best of both
**Status**: 💡 Proposed  
**Use case**: Optimal for most functions

```asm
; First few params in registers, rest on stack
; For func(a: u8, b: u8, c: u16, d: u16)
LD A, 10                ; param a in A
LD E, 20                ; param b in E  
LD HL, 300              ; param c in HL
PUSH DE                 ; param d on stack (if DE needed)
CALL func

func:
    ; A = param a (register)
    ; E = param b (register)
    ; HL = param c (register)
    ; Stack = param d
```

**Pros**:
- Fast common case
- Handles many params
- Flexible

**Cons**:
- Complex ABI
- Harder to implement
- Mixed access patterns

### 8. **Message Passing** - Memory blocks
**Status**: 💭 Concept  
**Use case**: Large structures, async operations

```asm
; Parameters in a "message block"
LD HL, param_block
LD (func_msg_ptr), HL
CALL func

param_block:
    DW 42               ; param 1
    DW 24               ; param 2
    DW return_addr      ; where to put result

func:
    LD HL, (func_msg_ptr)
    ; Read parameters from block
```

**Pros**:
- Unlimited data
- Async friendly
- Structure passing

**Cons**:
- Indirect access
- Memory bandwidth
- Complexity

## ABI Selection Strategy

### Compiler Heuristics

```rust
fn select_abi(func: &Function) -> ABI {
    match (func.param_count, func.total_param_size, func.properties) {
        // No parameters - use simplest
        (0, _, _) => ABI::Direct,
        
        // Few small params - use registers
        (1..=3, size, _) if size <= 6 => ABI::PhysicalRegister,
        
        // Interrupt handler - use shadow registers
        (_, _, props) if props.is_interrupt => ABI::ShadowRegister,
        
        // Recursive - must use stack
        (_, _, props) if props.is_recursive => ABI::StackFrame,
        
        // RAM-based, non-recursive - use SMC
        (_, _, props) if props.in_ram && !props.is_recursive => ABI::TrueSMC,
        
        // Default fallback
        _ => ABI::VirtualRegister,
    }
}
```

### Manual Override Annotations

```minz
@abi("register")        // Force register passing
@abi("stack")          // Force stack frame
@abi("smc")            // Force self-modifying code
@abi("shadow")         // Force shadow registers
@abi("virtual")        // Force virtual registers (memory)

// Detailed control
@abi("register: A=p1, HL=p2, stack=p3")
fun complex(p1: u8, p2: u16, p3: u32) -> u16 {
    // First param in A, second in HL, third on stack
}
```

## Register Preservation Rules

### Caller-saved (caller must preserve)
- A, HL, DE, BC (main working registers)
- Flags

### Callee-saved (function must preserve)
- IX, IY (if used)
- SP (stack pointer)
- Shadow registers (if convention doesn't use them)

### Special cases
- Interrupt handlers save ALL registers
- Leaf functions may preserve nothing
- SMC functions can modify anything

## Performance Comparison

| Convention | Call Overhead | Param Access | Best Use Case |
|-----------|--------------|--------------|---------------|
| SMC | 0 | Immediate (4T) | RAM, non-recursive |
| Physical Reg | 0 | Register (4T) | Small, fast functions |
| Shadow Reg | 8T (EXX) | Register (4T) | Interrupts |
| Virtual Reg | 0 | Memory (13T) | General purpose |
| Stack (IX) | 20T+ | IX+offset (19T) | Recursive, many params |
| Hybrid | 0-10T | Mixed (4-19T) | Optimal default |

## Future Considerations

### 1. **Vectored Parameters** - SIMD-style
```asm
; Multiple values in one register
LD HL, param1_param2    ; Two 8-bit params in HL
```

### 2. **Banker's ABI** - Page-based
```asm
; Parameters in banked memory
LD A, param_page
OUT (bank_port), A
LD HL, (param_addr)
```

### 3. **Continuation-Passing** - CPS style
```asm
; Pass continuation address
LD HL, continuation
PUSH HL
JP function             ; Function "returns" by JP (HL)
```

## Implementation Plan

### Phase 1 (Current)
- ✅ True SMC
- ✅ Virtual registers
- 🚧 Physical registers (partial)

### Phase 2 (v0.4.0)
- [ ] Stack frame (IX)
- [ ] Hybrid register+stack
- [ ] ABI annotations

### Phase 3 (Future)
- [ ] Shadow register ABI
- [ ] Message passing
- [ ] Advanced optimizations

## Conclusion

MinZ's flexible ABI system allows optimal performance for each use case. By analyzing functions and choosing appropriate calling conventions, MinZ can generate code that rivals hand-written assembly while maintaining high-level language benefits.

The key innovation is **adaptive ABI selection** - the compiler is smart enough to choose the best convention for each function, with manual override when needed.