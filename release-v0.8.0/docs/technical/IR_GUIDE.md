# MinZ Intermediate Representation (IR) Guide

## Overview

The MinZ compiler uses an Intermediate Representation (IR) as a bridge between the high-level AST (Abstract Syntax Tree) and the low-level Z80 assembly code. This IR serves several important purposes:

1. **Simplification**: Reduces complex language constructs to simpler operations
2. **Optimization**: Enables compiler optimizations independent of source language and target architecture
3. **Portability**: Makes it easier to target different architectures in the future
4. **Analysis**: Facilitates data flow analysis, register allocation, and other compiler passes

## IR Design Philosophy

The MinZ IR is designed to be:
- **Simple**: Few, orthogonal instructions
- **Explicit**: All operations are explicit (no hidden side effects)
- **SSA-friendly**: Can be easily converted to Static Single Assignment form
- **Low-level**: Close to machine operations but architecture-independent

## IR Instructions

### Control Flow Operations

```
OpNop         - No operation
OpLabel       - Define a label for jumps
OpJump        - Unconditional jump to label
OpJumpIf      - Jump to label if condition is true
OpJumpIfNot   - Jump to label if condition is false
OpCall        - Function call
OpReturn      - Return from function
```

### Data Movement

```
OpLoadConst   - Load immediate constant into register
OpLoadVar     - Load variable value into register
OpStoreVar    - Store register value to variable
OpLoadParam   - Load function parameter
OpLoadField   - Load struct field
OpStoreField  - Store to struct field
OpLoadIndex   - Load array element
OpStoreIndex  - Store to array element
```

### Arithmetic Operations

```
OpAdd         - Addition
OpSub         - Subtraction
OpMul         - Multiplication
OpDiv         - Division
OpMod         - Modulo
OpNeg         - Negation (unary minus)
```

### Bitwise Operations

```
OpAnd         - Bitwise AND
OpOr          - Bitwise OR
OpXor         - Bitwise XOR
OpNot         - Bitwise NOT
OpShl         - Shift left
OpShr         - Shift right
```

### Comparison Operations

```
OpEq          - Equal
OpNe          - Not equal
OpLt          - Less than
OpGt          - Greater than
OpLe          - Less than or equal
OpGe          - Greater than or equal
```

### Memory Operations

```
OpAlloc       - Allocate memory
OpFree        - Free memory
OpLoadPtr     - Load from pointer
OpStorePtr    - Store to pointer
```

## Virtual Registers

The IR uses an unlimited number of virtual registers, which are later mapped to physical Z80 registers or memory locations:

```go
r1, r2, r3, ... // Virtual registers
```

Special registers:
- `RegZero`: Always contains 0
- `RegSP`: Stack pointer
- `RegFP`: Frame pointer
- `RegRet`: Return value register

## IR Examples

### Example 1: Simple Arithmetic

MinZ source:
```minz
let x = 10;
let y = 20;
let z = x + y;
```

IR representation:
```
r1 = 10           // OpLoadConst
store x, r1       // OpStoreVar
r2 = 20           // OpLoadConst
store y, r2       // OpStoreVar
r3 = load x       // OpLoadVar
r4 = load y       // OpLoadVar
r5 = r3 + r4      // OpAdd
store z, r5       // OpStoreVar
```

### Example 2: Conditional Logic

MinZ source:
```minz
if x > 10 {
    y = 1;
} else {
    y = 0;
}
```

IR representation:
```
r1 = load x       // OpLoadVar
r2 = 10           // OpLoadConst
r3 = r1 > r2      // OpGt
jump_if_not r3, else_1
r4 = 1            // OpLoadConst
store y, r4       // OpStoreVar
jump end_if_1
else_1:
r5 = 0            // OpLoadConst
store y, r5       // OpStoreVar
end_if_1:
```

### Example 3: Function Call

MinZ source:
```minz
fn add(a: u8, b: u8) -> u8 {
    return a + b;
}

let result = add(5, 3);
```

IR representation:
```
; Function add
add:
    r1 = load_param 0    // First parameter (a)
    r2 = load_param 1    // Second parameter (b)
    r3 = r1 + r2         // OpAdd
    return r3            // OpReturn

; Call site
r4 = 5                   // OpLoadConst
r5 = 3                   // OpLoadConst
r6 = call add           // OpCall (args passed via convention)
store result, r6         // OpStoreVar
```

### Example 4: Loop

MinZ source:
```minz
let mut i = 0;
while i < 10 {
    i = i + 1;
}
```

IR representation:
```
r1 = 0                   // OpLoadConst
store i, r1              // OpStoreVar
loop_1:
r2 = load i              // OpLoadVar
r3 = 10                  // OpLoadConst
r4 = r2 < r3             // OpLt
jump_if_not r4, end_loop_1
r5 = load i              // OpLoadVar
r6 = 1                   // OpLoadConst
r7 = r5 + r6             // OpAdd
store i, r7              // OpStoreVar
jump loop_1
end_loop_1:
```

## IR to Z80 Translation

The IR instructions map to Z80 assembly as follows:

### Register Allocation
Virtual registers are mapped to:
1. Z80 registers (A, B, C, D, E, H, L, HL, DE, BC)
2. Stack locations (when registers are exhausted)

### Instruction Mapping Examples

```
IR: r1 = 42
Z80: LD A, 42
     LD (IX-offset), A

IR: r3 = r1 + r2
Z80: LD HL, (IX-offset1)  ; Load r1
     LD DE, (IX-offset2)  ; Load r2
     ADD HL, DE
     LD (IX-offset3), HL  ; Store to r3

IR: jump_if r1, label
Z80: LD A, (IX-offset)
     OR A
     JP NZ, label

IR: r1 = call func
Z80: CALL func
     LD (IX-offset), HL   ; Return value in HL
```

## Optimization Passes

The MinZ compiler implements several optimization passes that operate on the IR:

### 1. Constant Folding

Evaluates constant expressions at compile time:
```
; Before optimization
r1 = 10
r2 = 20
r3 = r1 + r2
store x, r3

; After constant folding
r1 = 30
store x, r1
```

The pass also handles:
- Arithmetic operations: `+`, `-`, `*`, `/`, `%`
- Bitwise operations: `&`, `|`, `^`, `<<`, `>>`
- Comparisons: `==`, `!=`, `<`, `>`, `<=`, `>=`
- Conditional jumps with constant conditions

### 2. Dead Code Elimination

Removes instructions that don't affect program output:
```
; Before optimization
r1 = 10
r2 = 20
r3 = r1 + r2    ; Result never used
return r1

; After dead code elimination
r1 = 10
return r1
```

Also removes:
- Unreachable code after returns/jumps
- Unused labels
- Redundant jumps to the next instruction

### 3. Peephole Optimization

Applies Z80-specific pattern matching for better code:

**Load Zero Optimization:**
```
; Before
r1 = 0           ; LD A, 0 (2 bytes, 7 cycles)

; After  
r1 = r1 ^ r1     ; XOR A, A (1 byte, 4 cycles)
```

**Increment/Decrement Optimization:**
```
; Before
r1 = 1
r2 = r3 + r1     ; ADD with constant 1

; After
r2 = inc r3      ; INC instruction (faster)
```

**Multiply by Power of 2:**
```
; Before
r1 = 8
r2 = r3 * r1     ; Multiply by 8

; After
r1 = 3
r2 = r3 << r1    ; Shift left by 3 (8 = 2^3)
```

### 4. Register Allocation

Maps virtual registers to Z80 physical registers using a linear scan algorithm:

- Computes live ranges for each virtual register
- Assigns physical registers based on usage patterns
- Spills to memory when necessary
- Considers 8-bit vs 16-bit operations

Available Z80 registers:
- 8-bit: A, B, C, D, E, H, L
- 16-bit pairs: BC, DE, HL

### 5. Function Inlining

Replaces small function calls with their bodies:
```
; Before inlining
r1 = 5
r2 = 3
r3 = call add    ; Function call overhead

; After inlining
r1 = 5
r2 = 3
r3 = r1 + r2     ; Direct computation
```

Inlining criteria:
- Function size < 10 instructions
- No recursive calls
- No loops in function body

## Future Enhancements

Planned improvements to the IR:

1. **SSA Form**: Convert to Static Single Assignment for better optimization
2. **Type Information**: Attach type information to operations
3. **Memory Model**: More sophisticated memory operations
4. **SIMD Operations**: Support for parallel operations where applicable
5. **Debug Information**: Preserve source location information

## Working with the IR

### Dumping IR

To see the IR for a MinZ program (when debug mode is enabled):
```bash
minzc -d program.minz
```

### Optimization Levels

The compiler supports different optimization levels:
- **No optimization** (default): Fast compilation, easier debugging
- **-O**: Basic optimizations (constant folding, dead code elimination)
- **-O** (full): All optimizations including peephole, register allocation, and inlining

Example:
```bash
minzc -O program.minz    # Enable optimizations
minzc -d -O program.minz # Debug output with optimizations
```

### IR Passes

The compiler performs these passes on the IR:
1. **Generation**: AST → IR
2. **Validation**: Ensure IR is well-formed
3. **Optimization**: Apply enabled optimization passes
4. **Register Allocation**: Assign virtual to physical registers
5. **Code Generation**: IR → Z80 assembly

### Writing New Optimization Passes

To add a new optimization pass:

1. Implement the `Pass` interface:
```go
type MyPass struct {}

func (p *MyPass) Name() string {
    return "My Optimization"
}

func (p *MyPass) Run(module *ir.Module) (bool, error) {
    changed := false
    for _, function := range module.Functions {
        // Transform function IR
        if optimizeFunction(function) {
            changed = true
        }
    }
    return changed, nil
}
```

2. Add to the optimizer configuration:
```go
// In optimizer.go
if level >= OptLevelBasic {
    opt.passes = append(opt.passes, NewMyPass())
}
```

3. Consider these guidelines:
   - Preserve program semantics
   - Track whether changes were made
   - Handle all IR instruction types
   - Test with the optimizer test suite

## Example: Complete Program Flow

Given this MinZ program:
```minz
fn factorial(n: u8) -> u16 {
    if n <= 1 {
        return 1;
    }
    return n * factorial(n - 1);
}
```

The compilation flow is:

1. **Parse** → AST
2. **Type Check** → Typed AST
3. **Generate IR**:
   ```
   factorial:
       r1 = load_param 0
       r2 = 1
       r3 = r1 <= r2
       jump_if_not r3, recurse
       r4 = 1
       return r4
   recurse:
       r5 = load_param 0
       r6 = 1
       r7 = r5 - r6
       r8 = call factorial
       r9 = load_param 0
       r10 = r9 * r8
       return r10
   ```
4. **Optimize** → Optimized IR
5. **Generate Z80** → Assembly code

This modular approach makes the compiler easier to understand, debug, and extend.