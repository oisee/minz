# Z80 Assembler Package

A complete Go-based Z80 assembler supporting the full Z80 instruction set, including undocumented opcodes. This assembler is designed for the MinZ compiler project to handle inline assembly blocks.

## Features

- **Full Z80 Instruction Set**: All documented Z80 instructions
- **Undocumented Instructions**: 
  - SLL (Shift Left Logical)
  - IX/IY half-register access (IXH, IXL, IYH, IYL)
  - Undocumented ED instructions
  - All prefix combinations (DD/FD CB sequences)
- **Complete Addressing Modes**: Register, immediate, indirect, indexed
- **Two-Pass Assembly**: Proper forward reference resolution
- **Directives**: ORG, DB, DW, DS, EQU, ALIGN
- **Symbol Table**: Label and constant management
- **Error Handling**: Detailed error messages with line numbers

## Usage

```go
import "minzc/pkg/z80asm"

// Create assembler
asm := z80asm.NewAssembler()

// Optional: Configure settings
asm.AllowUndocumented = true  // Default: true
asm.Strict = false           // Default: false

// Assemble source code
source := `
    ORG $8000
start:
    LD A, 42
    LD HL, message
    CALL print
    RET
    
message:
    DB "Hello, Z80!", 0
`

result, err := asm.AssembleString(source)
if err != nil {
    log.Fatal(err)
}

// Access results
fmt.Printf("Binary size: %d bytes\n", len(result.Binary))
fmt.Printf("Origin: $%04X\n", result.Origin)
fmt.Printf("Symbols: %v\n", result.Symbols)
```

## Supported Instructions

### Standard Instructions
- **Data Movement**: LD, EX, EXX, LDI, LDIR, LDD, LDDR
- **Arithmetic**: ADD, ADC, SUB, SBC, INC, DEC, NEG, DAA
- **Logic**: AND, OR, XOR, CP, CPL
- **Bit Operations**: BIT, SET, RES, RLC, RRC, RL, RR, SLA, SRA, SRL
- **Jumps**: JP, JR, DJNZ, CALL, RET, RST
- **Stack**: PUSH, POP
- **I/O**: IN, OUT, INI, INIR, IND, INDR, OUTI, OTIR, OUTD, OTDR
- **Control**: NOP, HALT, DI, EI, IM

### Undocumented Instructions
```asm
; SLL - Shift Left Logical (fills with 1)
SLL B           ; CB 30
SLL (IX+5)      ; DD CB 05 36

; IX/IY half registers
LD IXH, 10      ; DD 26 0A
LD IYL, B       ; FD 68
ADD A, IXH      ; DD 84
INC IXL         ; DD 2C

; Other undocumented
OUT (C), 0      ; ED 71
```

## Directives

```asm
ORG $8000       ; Set origin address
DB 1, 2, 3      ; Define bytes
DW $1234        ; Define words (little-endian)
DS 100, $FF     ; Define space (100 bytes of $FF)
LABEL: EQU 42   ; Define constant
ALIGN 256       ; Align to boundary
```

## Error Handling

The assembler provides detailed error messages:
- Undefined symbols
- Out of range values
- Invalid opcodes
- Syntax errors

## Testing

Comprehensive test suite compares output against sjasmplus:

```bash
go test ./pkg/z80asm
```

## Implementation Status

✅ Core assembler infrastructure
✅ Complete instruction encoding
✅ Undocumented instruction support  
✅ Two-pass assembly
✅ Symbol resolution
✅ Basic directives
⚠️  File I/O (currently string-based)
⚠️  Macro support (not implemented)
⚠️  Include files (not implemented)

## Design Notes

The assembler is designed to be:
- **Modular**: Separate parsing, encoding, and output stages
- **Extensible**: Easy to add new instructions or directives
- **Compatible**: Binary output matches sjasmplus
- **Embedded**: Can be used as a library in the MinZ compiler

## Performance

The assembler is optimized for correctness over speed. For typical inline assembly blocks in MinZ, performance is more than adequate.