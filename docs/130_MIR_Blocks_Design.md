# 130: MIR and Assembly Blocks - Low-Level Code Integration

## 🎯 Concept

MinZ provides two ways to write low-level code:
- **`asm`** - Z80 assembly (hardware-specific)
- **`mir`** - MinZ Intermediate Representation (CPU-independent)

Both can be used as entire functions or as inline blocks within MinZ code.

## 🏗️ Design Philosophy

### Consistent Syntax Rules
- **`@name(...)`** - Metafunctions (compile-time execution)
- **`keyword fun`** - Pure low-level functions
- **`keyword { }`** - Inline blocks within MinZ functions

## 📝 Syntax Overview

### Pure Functions
```minz
// Pure assembly function - no MinZ body
asm fun memcpy(dst: *u8, src: *u8, len: u16) -> void {
    LD E, (IX+4)    ; dst low
    LD D, (IX+5)    ; dst high
    LD L, (IX+6)    ; src low
    LD H, (IX+7)    ; src high
    LD C, (IX+8)    ; len low
    LD B, (IX+9)    ; len high
    LDIR            ; Block copy
    RET
}

// Pure MIR function - CPU-independent
mir fun strlen(str: *u8) -> u16 {
    load r1, [str]      ; String pointer
    load r2, #0         ; Counter
    
loop:
    load.u8 r3, [r1]    ; Load character
    jump.z r3, done     ; If zero, we're done
    inc r1              ; Next character
    inc r2              ; Increment count
    jump loop
    
done:
    return r2           ; Return count
}

// Regular MinZ function
fun calculate(x: u8, y: u8) -> u16 {
    return x * y + 42;
}
```

### Inline Blocks
```minz
fun optimized_copy(dst: *u8, src: *u8, len: u16) -> void {
    // MinZ code
    if len == 0 { return; }
    
    // Inline MIR block for performance
    mir {
        load r1, [src]      ; Load source pointer
        load r2, [dst]      ; Load destination pointer  
        load r3, [len]      ; Load length
        
    copy_loop:
        load.u8 r4, [r1]    ; Load byte from source
        store.u8 [r2], r4   ; Store to destination
        add r1, r1, #1      ; Increment source
        add r2, r2, #1      ; Increment destination
        sub r3, r3, #1      ; Decrement counter
        jump.nz copy_loop   ; Loop if not zero
    }
    
    // More MinZ code
    @print("Copy complete\n");  // Metafunction call
}

fun checksum_calc(data: *u8, len: u16) -> u8 {
    let sum: u8 = 0;
    
    // Inline assembly block for Z80-specific optimization
    asm {
        LD HL, (IX+4)   ; data pointer
        LD BC, (IX+6)   ; length
        XOR A           ; Clear accumulator
    checksum_loop:
        ADD A, (HL)     ; Add byte
        INC HL          ; Next byte
        DEC BC          ; Decrement counter
        LD E, A         ; Save A
        LD A, B         ; Check if BC == 0
        OR C
        LD A, E         ; Restore A
        JR NZ, checksum_loop
        LD (IX-1), A    ; Store to sum
    }
    
    return sum;
}
```

### Comparison Table
| Syntax | Meaning | Allows MinZ Code | Use Case |
|--------|---------|------------------|----------|
| `fun name()` | Regular MinZ function | Yes (required) | Normal code |
| `asm fun name()` | Pure assembly function | No | Hardware-specific routines |
| `mir fun name()` | Pure MIR function | No | Portable low-level code |
| `asm { }` | Assembly block in MinZ | Yes (surrounding) | Optimize critical sections |
| `mir { }` | MIR block in MinZ | Yes (surrounding) | Portable optimizations |
| `@name()` | Metafunction call | N/A | Compile-time operations |

## 📝 MIR Instruction Set

### Data Movement
```mir
load rX, [addr]         ; Load from memory
load rX, #immediate     ; Load immediate
load rX, rY            ; Register to register
store [addr], rX       ; Store to memory
move rX, rY            ; Move (alias for load)
```

### Arithmetic
```mir
add rX, rY, rZ         ; rX = rY + rZ
sub rX, rY, rZ         ; rX = rY - rZ
mul rX, rY, rZ         ; Multiply
div rX, rY, rZ         ; Divide
mod rX, rY, rZ         ; Modulo
inc rX                 ; Increment
dec rX                 ; Decrement
neg rX                 ; Negate
```

### Logical
```mir
and rX, rY, rZ         ; Bitwise AND
or rX, rY, rZ          ; Bitwise OR
xor rX, rY, rZ         ; Bitwise XOR
not rX, rY             ; Bitwise NOT
shl rX, rY, rZ         ; Shift left
shr rX, rY, rZ         ; Shift right
rol rX, rY, rZ         ; Rotate left
ror rX, rY, rZ         ; Rotate right
```

### Control Flow
```mir
jump label             ; Unconditional jump
jump.z label           ; Jump if zero
jump.nz label          ; Jump if not zero
jump.eq rX, rY, label  ; Jump if equal
jump.ne rX, rY, label  ; Jump if not equal
jump.lt rX, rY, label  ; Jump if less than
jump.gt rX, rY, label  ; Jump if greater than
call function          ; Function call
return                 ; Return from function
return rX              ; Return with value
```

### Type-Specific Operations
```mir
load.u8 rX, [addr]     ; Load unsigned 8-bit
load.i8 rX, [addr]     ; Load signed 8-bit
load.u16 rX, [addr]    ; Load unsigned 16-bit
load.i16 rX, [addr]    ; Load signed 16-bit
store.u8 [addr], rX    ; Store 8-bit
store.u16 [addr], rX   ; Store 16-bit
cast.u8 rX, rY         ; Cast to u8
cast.u16 rX, rY        ; Cast to u16
```

### Stack Operations
```mir
push rX                ; Push to stack
pop rX                 ; Pop from stack
push.all              ; Save all registers
pop.all               ; Restore all registers
```

### Special Operations
```mir
nop                   ; No operation
halt                  ; Stop execution
syscall #n            ; System call
smc.patch addr, value ; Self-modifying code patch
phi rX, [rY, rZ, ...] ; SSA phi node
```

## 🔧 Implementation

### Grammar Extension
```javascript
// grammar.js
mir_block: $ => seq(
    '@mir',
    '{',
    repeat($.mir_instruction),
    '}'
),

mir_instruction: $ => seq(
    optional(seq($.identifier, ':')),  // Label
    $.mir_opcode,
    optional($.mir_operands),
    optional(';', $.comment)
),

mir_opcode: $ => choice(
    'load', 'store', 'move',
    'add', 'sub', 'mul', 'div',
    'and', 'or', 'xor', 'not',
    'jump', 'call', 'return',
    // ... etc
),

mir_operand: $ => choice(
    $.mir_register,
    $.mir_memory,
    $.mir_immediate,
    $.identifier  // Label/function
),

mir_register: $ => /r[0-9]+/,
mir_memory: $ => seq('[', $.expression, ']'),
mir_immediate: $ => seq('#', $.number)
```

### Semantic Analysis
```go
type MIRBlock struct {
    Instructions []MIRInstruction
    UsedRegisters map[int]bool
    Labels map[string]int
}

type MIRInstruction struct {
    Label    string
    Opcode   MIROpcode
    Operands []MIROperand
    Type     Type  // For type-specific ops
}

func (a *Analyzer) analyzeMIRBlock(block *ast.MIRBlock) (*MIRBlock, error) {
    mir := &MIRBlock{
        UsedRegisters: make(map[int]bool),
        Labels: make(map[string]int),
    }
    
    // Verify instruction validity
    for i, inst := range block.Instructions {
        if inst.Label != "" {
            mir.Labels[inst.Label] = i
        }
        
        // Type check operands
        if err := a.checkMIROperands(inst); err != nil {
            return nil, err
        }
        
        // Track register usage
        a.trackRegisterUsage(inst, mir.UsedRegisters)
    }
    
    return mir, nil
}
```

### Code Generation
```go
func (g *Z80Generator) generateMIRBlock(mir *MIRBlock) error {
    // Map abstract registers to Z80 registers
    regMap := g.allocateMIRRegisters(mir.UsedRegisters)
    
    for _, inst := range mir.Instructions {
        switch inst.Opcode {
        case MIR_LOAD:
            g.generateLoad(inst, regMap)
        case MIR_ADD:
            g.generateAdd(inst, regMap)
        case MIR_JUMP:
            g.generateJump(inst)
        // ... etc
        }
    }
    
    return nil
}

// Example: MIR load -> Z80
func (g *Z80Generator) generateLoad(inst MIRInstruction, regMap map[int]string) {
    mirReg := inst.Operands[0].(MIRRegister)
    z80Reg := regMap[mirReg.Number]
    
    switch src := inst.Operands[1].(type) {
    case MIRImmediate:
        g.emit("LD %s, %d", z80Reg, src.Value)
    case MIRMemory:
        g.emit("LD %s, (%s)", z80Reg, src.Address)
    }
}
```

## 💡 Use Cases

### 1. Platform-Independent Optimization
```minz
fun strlen_mir(str: *u8) -> u16 {
    @mir {
        load r1, [str]      ; String pointer
        load r2, #0         ; Counter
        
    loop:
        load.u8 r3, [r1]    ; Load character
        jump.z r3, done     ; If zero, we're done
        inc r1              ; Next character
        inc r2              ; Increment count
        jump loop
        
    done:
        return r2           ; Return count
    }
}
```

### 2. Complex Control Flow
```minz
fun switch_mir(value: u8) -> u8 {
    @mir {
        load r1, [value]
        
        ; Jump table approach
        jump.eq r1, #1, case1
        jump.eq r1, #2, case2
        jump.eq r1, #3, case3
        jump default
        
    case1:
        load r2, #10
        jump end
        
    case2:
        load r2, #20
        jump end
        
    case3:
        load r2, #30
        jump end
        
    default:
        load r2, #0
        
    end:
        return r2
    }
}
```

### 3. SSA-Style Code
```minz
fun max_mir(a: u8, b: u8) -> u8 {
    @mir {
        load r1, [a]
        load r2, [b]
        
        jump.gt r1, r2, a_greater
        
        ; b is greater or equal
        move r3, r2
        jump end
        
    a_greater:
        move r3, r1
        
    end:
        ; SSA phi node (r3 comes from either path)
        return r3
    }
}
```

### 4. Self-Modifying Code in MIR
```minz
fun adaptive_loop(count: u8) -> void {
    @mir {
        load r1, [count]
        
        ; Patch the loop counter directly
        smc.patch loop_count, r1
        
    loop:
        ; Do something
        nop
        
    loop_count:
        load r2, #0  ; This gets patched!
        dec r2
        store [loop_count+1], r2  ; Self-modify
        jump.nz loop
        
        return
    }
}
```

## 🎯 Benefits

1. **CPU Independence** - Write once, compile to any target
2. **Optimization Opportunities** - Easier to optimize at MIR level
3. **Educational** - Learn compiler internals
4. **Debugging** - Inspect intermediate representation
5. **Portability** - Future targets (6502, 6809, eZ80, etc.)

## 🔄 MIR Pipeline

```
MinZ Source
    ↓
Parser (AST)
    ↓
Semantic Analysis
    ↓
MIR Generation ← @mir blocks injected here
    ↓
MIR Optimization
    ↓
Target Code Generation (Z80, 6502, etc.)
    ↓
Assembly Output
```

## 📊 MIR vs Direct Assembly

| Aspect | @asm | @mir |
|--------|------|------|
| Portability | Z80 only | Any CPU |
| Optimization | Manual | Automatic |
| Register Allocation | Manual | Automatic |
| Type Safety | None | Type-aware |
| Debugging | Hard | Easier |
| Performance | Optimal* | Near-optimal |

*When hand-optimized by expert

## 🚀 Future Extensions

### MIR Intrinsics
```minz
@mir {
    intrinsic.memcpy r1, r2, r3   ; Built-in operations
    intrinsic.memset r1, r2, r3
    intrinsic.strlen r1
}
```

### Vector Operations
```minz
@mir {
    load.v4 v1, [array]    ; Load 4 bytes
    add.v4 v2, v1, v1      ; SIMD-style add
    store.v4 [result], v2
}
```

### Parallel Hints
```minz
@mir {
    parallel {
        load r1, [a]
        load r2, [b]
        load r3, [c]
    }
    ; Compiler can reorder/optimize
}
```

## 📚 Examples

### Complete Function in MIR
```minz
fun bubble_sort_mir(arr: *u8, len: u8) -> void {
    @mir {
        load r1, [arr]      ; Array pointer
        load r2, [len]      ; Length
        
    outer_loop:
        load r3, #0         ; i = 0
        load r4, r2         ; limit = len
        dec r4              ; limit = len - 1
        
    inner_loop:
        ; Load arr[i] and arr[i+1]
        move r5, r1
        add r5, r5, r3      ; r5 = arr + i
        load.u8 r6, [r5]    ; r6 = arr[i]
        inc r5
        load.u8 r7, [r5]    ; r7 = arr[i+1]
        
        ; Compare and swap if needed
        jump.le r6, r7, no_swap
        
        ; Swap
        store.u8 [r5], r6   ; arr[i+1] = r6
        dec r5
        store.u8 [r5], r7   ; arr[i] = r7
        
    no_swap:
        inc r3              ; i++
        jump.lt r3, r4, inner_loop
        
        dec r2              ; len--
        jump.nz r2, outer_loop
        
        return
    }
}
```

---

*@mir blocks - Write once, run on any 8-bit CPU!*