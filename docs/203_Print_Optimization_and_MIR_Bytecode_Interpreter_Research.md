# Research: @print Optimization Issue & MIR Bytecode Interpreter for Z80

## Part 1: Root Cause Analysis - @print Working but Not Visible

### Issue Report
User Raúl reported that `@print` statements don't appear in compile-time messages or generated assembly. However, investigation shows **@print IS working correctly**.

### Evidence

**Test Code:**
```minz
fn main() -> u8 {
    @print("Hello from compile time!");
    let x = 42;
    @print("Value of x: ", x);
    return 0;
}
```

**Generated Assembly (Lines 28-30, 39-40):**
```asm
; Print "Hello from compile time!" (24 chars via loop)
CALL print_string
; ...
; Print "Value of x: " (12 chars via loop)
CALL print_string
```

### The Real Problem: Visibility vs Functionality

The confusion stems from **two different types of @print**:

1. **Compile-Time @print** - Should output during compilation
2. **Runtime @print** - Should generate print code in assembly

Currently, MinZ is treating `@print` as a **runtime metafunction** that generates print code, NOT as a compile-time output function.

### Why This Happens

```go
// In purity.go - Functions with side effects marked as impure
case ir.OpPrint:
    // Print is a side effect - marked as impure
    return Impure
```

But `@print` is special - it's a **metafunction** that should:
1. Print at compile time (for debugging)
2. Optionally generate runtime print code

### Solution Design

```go
// Enhanced metafunction handling
type MetafunctionHandler struct {
    CompileTimeOutput bool  // Print during compilation
    RuntimeGeneration bool  // Generate runtime code
}

func handlePrint(args []Value) {
    // 1. Always output at compile time
    fmt.Printf("[COMPILE TIME]: %v\n", args)
    
    // 2. Optionally generate runtime code
    if config.GenerateRuntimePrints {
        generatePrintInstruction(args)
    }
}
```

### Recommended Fix

1. **Immediate**: Add compile-time output for @print
2. **Better**: Separate @compile_print (compile only) from @runtime_print (runtime only)
3. **Best**: Context-aware @print that does both

---

## Part 2: MIR Bytecode Interpreter on Z80 - Revolutionary Concept

### The Vision: MIR VM Running ON Z80 Hardware

Not a MIR VM for modern systems, but a **MIR interpreter that runs on actual Z80**!

```
MinZ Source → MIR Bytecode → Z80 MIR Interpreter → Execution on Z80
```

### Why This Is Brilliant

1. **Universal Z80 Binary**: One interpreter + MIR bytecode = runs anywhere
2. **Dynamic Loading**: Load different MIR programs without recompilation
3. **Smaller Programs**: MIR bytecode more compact than native Z80
4. **Debugging**: Step through MIR instructions on real hardware

### MIR Bytecode Format for Z80

```asm
; Compact MIR bytecode format (1-3 bytes per instruction)
; [opcode:8][arg1:8?][arg2:8?]

MIR_NOP     = $00
MIR_PUSH8   = $01  ; [op][value]
MIR_PUSH16  = $02  ; [op][low][high]
MIR_POP     = $03
MIR_ADD     = $04
MIR_SUB     = $05
MIR_MUL     = $06
MIR_CALL    = $10  ; [op][addr_low][addr_high]
MIR_RET     = $11
MIR_JMP     = $12  ; [op][offset]
MIR_JZ      = $13  ; [op][offset]
MIR_PRINT   = $20  ; Print TOS
```

### Z80 MIR Interpreter Implementation

```asm
; MIR VM for Z80 (runs on actual hardware!)
; Uses: IX = instruction pointer, IY = stack pointer

MIR_VM_START:
    LD IX, MIR_PROGRAM  ; IX points to MIR bytecode
    LD IY, MIR_STACK    ; IY points to stack
    
MIR_VM_LOOP:
    LD A, (IX)          ; Fetch opcode
    INC IX              ; Advance IP
    
    ; Dispatch based on opcode
    CP MIR_NOP
    JR Z, OP_NOP
    CP MIR_PUSH8
    JR Z, OP_PUSH8
    CP MIR_ADD
    JR Z, OP_ADD
    CP MIR_CALL
    JR Z, OP_CALL
    CP MIR_PRINT
    JR Z, OP_PRINT
    ; ... more opcodes
    
    ; Unknown opcode - halt
    HALT

OP_NOP:
    JR MIR_VM_LOOP

OP_PUSH8:
    LD A, (IX)          ; Get value
    INC IX
    LD (IY), A          ; Push to stack
    INC IY
    JR MIR_VM_LOOP

OP_ADD:
    DEC IY              ; Pop first value
    LD A, (IY)
    DEC IY              ; Pop second value
    LD B, (IY)
    ADD A, B            ; Add
    LD (IY), A          ; Push result
    INC IY
    JR MIR_VM_LOOP

OP_PRINT:
    DEC IY              ; Pop value
    LD A, (IY)
    CALL PRINT_A        ; Use ROM routine
    JR MIR_VM_LOOP

OP_CALL:
    ; Save current IX on return stack
    LD L, (IX)          ; Get call address low
    INC IX
    LD H, (IX)          ; Get call address high
    INC IX
    PUSH IX             ; Save return address
    LD IX, HL           ; Jump to function
    JR MIR_VM_LOOP
```

### Example: Fibonacci in MIR Bytecode

```minz
// Original MinZ
fn fib(n: u8) -> u8 {
    if n <= 1 { return n }
    return fib(n-1) + fib(n-2)
}
```

```hex
; MIR bytecode for fib function
FIB_MIR:
    DB MIR_PUSH8, 1     ; Push 1
    DB MIR_CMP_LE       ; Compare n <= 1
    DB MIR_JZ, 5        ; Jump if false
    DB MIR_DUP          ; Return n
    DB MIR_RET
    DB MIR_DUP          ; Duplicate n
    DB MIR_PUSH8, 1     ; Push 1
    DB MIR_SUB          ; n-1
    DB MIR_CALL         ; Call fib(n-1)
    DW FIB_MIR
    DB MIR_SWAP         ; Swap result with n
    DB MIR_PUSH8, 2     ; Push 2
    DB MIR_SUB          ; n-2
    DB MIR_CALL         ; Call fib(n-2)
    DW FIB_MIR
    DB MIR_ADD          ; Add results
    DB MIR_RET
```

### Performance Analysis

| Approach | Size | Speed | Flexibility |
|----------|------|-------|------------|
| Native Z80 | 100% | 100% | Static |
| MIR Interpreter | 40-60% | 10-20% | Dynamic |
| Threaded Code | 60-80% | 30-40% | Semi-dynamic |
| JIT (impossible) | N/A | N/A | N/A |

### Advanced Features

#### 1. Threaded Code Optimization
```asm
; Instead of bytecode dispatch, use direct threading
PROGRAM:
    DW OP_PUSH8_IMPL
    DB 42
    DW OP_PRINT_IMPL
    DW OP_RET_IMPL

; Each operation ends with:
    LD HL, (IX)     ; Get next operation address
    INC IX
    INC IX
    JP (HL)         ; Jump directly to it
```

#### 2. Hybrid Execution
```asm
; Mix interpreted MIR with native code
MIR_NATIVE_CALL:
    DB MIR_NATIVE       ; Special opcode
    DW NATIVE_FUNCTION  ; Address of Z80 code
    ; Interpreter calls native directly
```

#### 3. MIR-to-Z80 Caching
```asm
; First execution: interpret and cache translation
; Second execution: run cached native code
MIR_CACHE:
    ; 256-byte cache for hot functions
    DS 256
```

### Implementation Phases

#### Phase 1: Basic Stack Machine (1 week)
- Stack operations (push, pop, dup, swap)
- Arithmetic (add, sub, mul, div)
- Control flow (jump, conditional jump, call, return)
- I/O (print, input)

#### Phase 2: Memory & Variables (1 week)
- Load/store operations
- Local variable frame
- Global variable access
- Array indexing

#### Phase 3: Optimizations (2 weeks)
- Threaded code dispatch
- Inline caching for calls
- Peephole optimization in bytecode
- Native function interface

#### Phase 4: Development Tools
- MIR assembler for Z80
- Bytecode debugger
- Performance profiler
- Standard library in MIR

### Use Cases

#### 1. Educational Tool
```minz
// Students write MinZ, runs on real Z80 hardware via interpreter
fn learn_z80() {
    @print("No assembly required!")
}
```

#### 2. Rapid Development
```bash
# Compile to MIR bytecode
mz program.minz -b mir -o program.mir

# Load on Z80 with interpreter
LOAD "mirvm.tap"
LOAD "program.mir"
RUN
```

#### 3. Universal Z80 Programs
One MIR interpreter binary works on:
- ZX Spectrum
- Amstrad CPC
- MSX
- TI-83 calculators
- CP/M systems

### The Ultimate Vision: Self-Hosting

```minz
// MIR interpreter... written in MinZ!
fn mir_interpret(bytecode: []u8) {
    let ip = 0
    let sp = 0
    let stack: [u8; 256]
    
    loop {
        when bytecode[ip] {
            MIR_PUSH8 => {
                stack[sp] = bytecode[ip+1]
                sp += 1
                ip += 2
            }
            MIR_ADD => {
                sp -= 1
                stack[sp-1] += stack[sp]
                ip += 1
            }
            // ...
        }
    }
}
```

Compile this to MIR, run on MIR interpreter = **MIR running MIR**!

---

## Part 3: Comparison - Native vs Bytecode

### When to Use Native Z80 Compilation
- Performance critical code
- Hardware manipulation
- Interrupt handlers
- Size-optimized programs

### When to Use MIR Bytecode
- Portability across Z80 systems
- Dynamic program loading
- Educational/learning
- Rapid prototyping
- Complex algorithms where size > speed

### Hybrid Approach: Best of Both Worlds

```minz
// Performance critical - compile to native
@native
fn fast_graphics() { /* ... */ }

// Portable logic - compile to MIR
@bytecode  
fn game_logic() { /* ... */ }

// Let compiler decide
fn main() {
    fast_graphics()  // Inlined native code
    game_logic()     // Call to MIR bytecode
}
```

---

## Recommendations

### For @print Issue
1. **Quick Fix**: Add console output during compilation
2. **Long-term**: Implement proper compile-time vs runtime distinction

### For MIR Bytecode Interpreter
1. **Proof of Concept**: Basic stack machine in 200-300 bytes
2. **Practical Version**: Full interpreter in ~2KB
3. **Educational Package**: MinZ + MIR VM as learning platform

### Revolutionary Potential
Combining both concepts:
- **Compile-time @print** for development feedback
- **MIR bytecode** with built-in print for universal execution
- **One toolchain** from modern development to vintage hardware

This isn't just fixing a bug or adding a feature - it's creating a **new paradigm for retro computing**: write once in MinZ, run anywhere through MIR, with full development visibility through proper @print handling.

The Z80 becomes not just a target, but a **platform for portable bytecode execution** - 40 years before Java's "write once, run anywhere"!