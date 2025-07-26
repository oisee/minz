# MinZ Inline Assembly Design

## Overview

MinZ inline assembly provides a thin layer over raw Z80 assembly, performing only minimal label resolution before emitting code directly to the output .a80 file for sjasmplus to assemble.

## Syntax

### Basic Inline Assembly Block
```minz
fun set_pixel(x: u8, y: u8) -> void {
    let addr: u16 = calculate_address(x, y);
    let mask: u8 = 1 << (x & 7);
    
    asm {
        ld hl, !addr     ; !addr references MinZ variable
        ld a, (hl)
        or !mask         ; !mask references MinZ variable  
        ld (hl), a
    }
}
```

### Named Assembly Blocks
```minz
// Named block for reusable assembly routines
asm fast_clear {
    ld hl, $4000
    ld de, $4001
    ld bc, $1AFF
    ld (hl), 0
    ldir
    ret
}

fun clear_screen() -> void {
    asm {
        call !fast_clear   ; Reference named asm block
    }
}
```

### Cross-referencing Between Assembly Blocks
```minz
asm init_routine {
    ld a, 7
    out ($FE), a
loop_start:
    dec a
    jr nz, loop_start
    ret
}

asm other_routine {
    call !init_routine.loop_start  ; Reference label in another asm block
}
```

## Label Resolution Rules

### 1. MinZ Symbol References (! prefix)
- `!variable` - Resolves to the address of a MinZ variable
- `!function` - Resolves to the entry point of a MinZ function
- `!constant` - Resolves to the value of a MinZ constant
- `!asm_block` - Resolves to the start of a named asm block
- `!asm_block.label` - Resolves to a label within a named asm block

### 2. Local Labels (no prefix)
- Labels without prefix are local to the current asm block
- They're emitted as-is to the output file
- sjasmplus handles these normally

### 3. Resolution Process
1. Scanner identifies `!symbol` references in asm blocks
2. Symbol table lookup determines the type and address/value
3. Compiler replaces `!symbol` with actual address/value
4. Everything else passes through unchanged

## Implementation in Compiler

### AST Addition
```go
type AsmStmt struct {
    Name   string   // Optional name for named blocks
    Code   string   // Raw assembly code
    Pos    Position
}
```

### Code Generation
```go
func (g *Generator) emitAsm(asm *ir.AsmBlock) {
    // Process line by line
    lines := strings.Split(asm.Code, "\n")
    for _, line := range lines {
        processed := g.resolveAsmSymbols(line)
        g.emit(processed)
    }
}

func (g *Generator) resolveAsmSymbols(line string) string {
    // Simple regex to find !symbol references
    return symbolRegex.ReplaceAllStringFunc(line, func(match string) string {
        symbol := strings.TrimPrefix(match, "!")
        
        // Resolve symbol from symbol table
        if addr, ok := g.symbolTable[symbol]; ok {
            return fmt.Sprintf("$%04X", addr)
        }
        
        // Handle dotted references like !block.label
        if strings.Contains(symbol, ".") {
            parts := strings.Split(symbol, ".")
            if blockAddr, ok := g.asmBlockLabels[parts[0]][parts[1]]; ok {
                return fmt.Sprintf("$%04X", blockAddr)
            }
        }
        
        // If not found, emit error
        g.error("Unknown symbol: " + symbol)
        return match
    })
}
```

## Output Example

Input MinZ:
```minz
const SCREEN: u16 = $4000;
let counter: u8 = 0;

fun increment() -> void {
    asm {
        ld hl, !counter
        inc (hl)
    }
}

asm clear_byte {
    xor a
loop:
    ld (hl), a
    inc hl
    djnz loop
    ret
}
```

Output .a80:
```asm
; Generated from MinZ
    ORG $8000

SCREEN  EQU $4000
counter: DB 0

increment:
    ; inline asm block
    ld hl, counter
    inc (hl)
    ret

clear_byte:
    xor a
loop:
    ld (hl), a
    inc hl  
    djnz loop
    ret
```

## Benefits

1. **Minimal Complexity**: No need to implement an assembler
2. **Full Z80 Power**: All sjasmplus features available
3. **Clean Integration**: MinZ symbols accessible from assembly
4. **Readable Output**: Generated .a80 remains human-readable
5. **Debugging Friendly**: Assembly maps directly to source

## Limitations

1. No syntax checking of assembly code (left to sjasmplus)
2. No register allocation coordination between MinZ and asm
3. No automatic stack management in asm blocks
4. Type safety ends at the asm boundary

## Usage Guidelines

1. Use inline asm for:
   - Hardware access (I/O ports)
   - Performance-critical loops
   - Interrupt handlers
   - Self-modifying code

2. Keep asm blocks small and focused
3. Document register usage and side effects
4. Prefer MinZ code when possible

## Future Enhancements

1. Register constraint syntax (like GCC):
   ```minz
   asm {
       "ld a, !0"
       : "=a"(result)
       : "r"(input)
       : "a"
   }
   ```

2. Automatic label generation for unnamed blocks
3. Integration with optimizer (mark clobbered registers)
4. Source-level debugging information preservation