# MinZ to Z80 Compiler Architecture

## Overview

The MinZ compiler will translate MinZ source code to Z80 assembly in sjasmplus `.a80` format.

## Compilation Pipeline

1. **Parsing** (tree-sitter) → AST
2. **Semantic Analysis** → Typed AST
3. **IR Generation** → MinZ IR
4. **Optimization** → Optimized IR
5. **Code Generation** → Z80 Assembly
6. **Assembly Output** → `.a80` file

## Key Components

### 1. Frontend (minzc)
- Parse MinZ source using tree-sitter binding
- Build symbol table
- Type checking and inference
- Generate IR

### 2. Intermediate Representation (IR)
```
// Example IR operations
LOAD_CONST r1, 42
LOAD_VAR r2, "x"
ADD r3, r1, r2
STORE_VAR "y", r3
```

### 3. Register Allocator
- Map IR registers to Z80 registers (A, B, C, D, E, H, L)
- Spill to stack when needed
- Special handling for HL, DE, BC pairs

### 4. Code Generator
Maps IR to Z80 instructions:
- `LOAD_CONST r, n` → `LD r, n`
- `ADD r1, r2, r3` → `LD A, r2; ADD A, r3; LD r1, A`
- Function calls → `CALL` with stack setup
- Structs → Memory layout with offsets

### 5. Memory Layout
```
$0000-$3FFF: ROM (if targeting cartridge)
$4000-$7FFF: RAM
$8000-$BFFF: Paged RAM/ROM
$C000-$FFFF: System/Stack
```

### 6. Output Format (.a80)
```asm
; MinZ generated code for: main.minz
; Generated: 2024-01-20

    ORG $8000

; Constants
MAX_SCORE: EQU 9999

; Data section
player_pos:
    DW 128, 96  ; x, y

; Code section
main:
    ; Initialize
    LD HL, player_pos
    LD (HL), 128
    INC HL
    LD (HL), 96
    
    ; Main loop
.loop:
    CALL read_input
    OR A
    JR Z, .exit
    
    ; ... game logic ...
    
    JR .loop
    
.exit:
    RET

; Function: read_input
read_input:
    IN A, ($FE)
    RET

    END main
```

## Implementation Language

The compiler is written in **Go** with tree-sitter integration via S-expressions.

## Current Implementation Status

### Completed Features
1. ✅ Tree-sitter grammar and parser
2. ✅ S-expression to AST conversion
3. ✅ Symbol table and type checker
4. ✅ IR generation with SSA-style registers
5. ✅ Advanced register allocation (physical → shadow → memory)
6. ✅ Function calls with multiple calling conventions
7. ✅ Self-modifying code (SMC) support
8. ✅ TSMC references (parameters as immediate operands)
9. ✅ Basic optimizations (constant folding, dead code elimination)
10. ✅ Assignment statements with mutability checking
11. ✅ Type-aware code generation (u8 vs u16)

### Advanced Features
- **TSMC (Tree-Structured Machine Code)**: Parameters become self-modifying immediate operands
- **Shadow Register Optimization**: Automatic use of Z80's alternate register set
- **SMC Calling Convention**: Functions modify their own code for performance
- **Hierarchical Register Allocation**: Physical → Shadow → Memory spilling

### Next Steps
1. Complex assignment targets (arrays, structs)
2. Compound assignments (+=, -=)
3. Auto-dereferencing
4. Advanced TSMC patterns
5. Module system implementation