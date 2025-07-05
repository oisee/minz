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

The compiler will be written in:
- **Rust** (recommended) - Performance, safety, good AST manipulation
- **TypeScript** - Easy tree-sitter integration, good tooling
- **C** - Direct tree-sitter integration, maximum performance

## Next Implementation Steps

1. Create basic CLI tool that reads MinZ and outputs AST
2. Implement symbol table and type checker
3. Design and implement IR
4. Create code generator for basic operations
5. Add register allocation
6. Implement function calls and stack management
7. Add struct/array support
8. Optimize generated code
9. Add debugging information