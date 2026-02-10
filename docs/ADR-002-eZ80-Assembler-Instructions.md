# ADR-002: eZ80 Assembler Instruction Support

**Status:** Accepted
**Date:** 2026-02-10
**Author:** MinZ Team

## Context

The MinZ assembler (MZA) currently supports Z80 instructions. To support the Agon Light 2 (eZ80), we need to add eZ80-specific instructions and ADL mode suffixes.

## Decision

### 1. Reference Implementation

Use [agon-ez80asm](https://github.com/AgonPlatform/agon-ez80asm) (MIT License) as reference for:
- Instruction encoding
- ADL suffix handling
- eZ80-specific opcodes

### 2. ADL Suffix Prefixes

| Suffix | Opcode | Meaning |
|--------|--------|---------|
| `.SIS` | 0x40 | Short Instruction, Short operands (Z80 compat) |
| `.LIS` | 0x49 | Long Instruction, Short operands |
| `.SIL` | 0x52 | Short Instruction, Long operands |
| `.LIL` | 0x5B | Long Instruction, Long operands (native ADL) |

Short forms:
- `.S` = `.SIS` (if ADL=0) or `.SIL` (if ADL=1)
- `.L` = `.LIS` (if ADL=0) or `.LIL` (if ADL=1)

### 3. New eZ80 Instructions

#### LEA - Load Effective Address
```asm
LEA BC, IX+d    ; ED 02 dd
LEA DE, IX+d    ; ED 12 dd
LEA HL, IX+d    ; ED 22 dd
LEA IX, IX+d    ; ED 32 dd
LEA IY, IX+d    ; ED 55 dd
LEA BC, IY+d    ; ED 03 dd
LEA DE, IY+d    ; ED 13 dd
LEA HL, IY+d    ; ED 23 dd
LEA IX, IY+d    ; ED 54 dd
LEA IY, IY+d    ; ED 33 dd
```

#### PEA - Push Effective Address
```asm
PEA IX+d        ; ED 65 dd
PEA IY+d        ; ED 66 dd
```

#### MLT - 8x8 Multiply
```asm
MLT BC          ; ED 4C  (BC = B * C)
MLT DE          ; ED 5C  (DE = D * E)
MLT HL          ; ED 6C  (HL = H * L)
MLT SP          ; ED 7C  (SP = SPH * SPL)
```

#### Control Instructions
```asm
SLP             ; ED 76  (Sleep/low power)
STMIX           ; ED 7D  (Set mixed ADL mode)
RSMIX           ; ED 7E  (Reset mixed ADL mode)
```

#### Extended Block I/O
```asm
; Input
IND2    ; ED 8C    INI2    ; ED 84
IND2R   ; ED 9C    INI2R   ; ED 94
INDM    ; ED 8A    INIM    ; ED 82
INDMR   ; ED 9A    INIMR   ; ED 92
INDRX   ; ED CA    INIRX   ; ED C2

; Output
OTD2R   ; ED BC    OTI2R   ; ED B4
OTDM    ; ED 8B    OTIM    ; ED 83
OTDMR   ; ED 9B    OTIMR   ; ED 93
OTDRX   ; ED CB    OTIRX   ; ED C3
OUTD2   ; ED AC    OUTI2   ; ED A4

; Port 0 I/O
IN0 r, (n)      ; ED 00+r*8 nn
OUT0 (n), r     ; ED 01+r*8 nn
```

### 4. Implementation in MZA

Add to `instruction_table.go`:

```go
// ADL suffix codes
const (
    CODE_SIS = 0x40
    CODE_LIS = 0x49
    CODE_SIL = 0x52
    CODE_LIL = 0x5B
)

// Add new operand types
const (
    OpTypeImm24    OpType = iota + 100  // 24-bit immediate
    OpTypeIndImm24                       // (mmnn) 24-bit address
)

// LEA instructions
var leaInstructions = []InstructionPattern{
    {Mnemonic: "LEA", Operands: []OperandPattern{
        {OpTypeReg16, "BC"}, {OpTypeIndIdx, ""}},
        EncodingFunc: encodeLEA, Encoding: []byte{0xED, 0x02}},
    // ... more patterns
}

// PEA instructions
var peaInstructions = []InstructionPattern{
    {Mnemonic: "PEA", Operands: []OperandPattern{
        {OpTypeIndIdx, ""}},
        EncodingFunc: encodePEA, Encoding: []byte{0xED, 0x65}},
}

// MLT instructions
var mltInstructions = []InstructionPattern{
    {Mnemonic: "MLT", Operands: []OperandPattern{
        {OpTypeReg16, "BC"}}, Encoding: []byte{0xED, 0x4C}},
    {Mnemonic: "MLT", Operands: []OperandPattern{
        {OpTypeReg16, "DE"}}, Encoding: []byte{0xED, 0x5C}},
    {Mnemonic: "MLT", Operands: []OperandPattern{
        {OpTypeReg16, "HL"}}, Encoding: []byte{0xED, 0x6C}},
    {Mnemonic: "MLT", Operands: []OperandPattern{
        {OpTypeReg16, "SP"}}, Encoding: []byte{0xED, 0x7C}},
}
```

### 5. Suffix Handling

```go
func (a *Assembler) handleADLSuffix(mnemonic string) (string, byte) {
    // Check for .SIS, .LIS, .SIL, .LIL, .S, .L suffixes
    suffixes := map[string]byte{
        ".SIS": CODE_SIS, ".LIS": CODE_LIS,
        ".SIL": CODE_SIL, ".LIL": CODE_LIL,
    }

    for suffix, code := range suffixes {
        if strings.HasSuffix(strings.ToUpper(mnemonic), suffix) {
            base := mnemonic[:len(mnemonic)-len(suffix)]
            return base, code
        }
    }

    // Handle .S and .L based on current ADL mode
    if strings.HasSuffix(strings.ToUpper(mnemonic), ".S") {
        base := mnemonic[:len(mnemonic)-2]
        if a.adlMode { return base, CODE_SIL }
        return base, CODE_SIS
    }
    if strings.HasSuffix(strings.ToUpper(mnemonic), ".L") {
        base := mnemonic[:len(mnemonic)-2]
        if a.adlMode { return base, CODE_LIL }
        return base, CODE_LIS
    }

    return mnemonic, 0
}
```

### 6. Future: External Assembler Option

Consider using agon-ez80asm as external tool:
```bash
# Option A: Built-in MZA with eZ80 support
minzc program.minz -t agon

# Option B: External assembler (future)
minzc program.minz -t agon --asm=ez80asm
```

## Consequences

### Positive
- Full eZ80 instruction set support
- Proper ADL mode suffix handling
- MIT-licensed reference available
- Can leverage tested implementation

### Negative
- Significant addition to instruction table
- Need comprehensive testing
- Mode tracking complexity

## References

- [eZ80 CPU User Manual](https://www.zilog.com/docs/um0077.pdf) - Official documentation
- [agon-ez80asm](https://github.com/AgonPlatform/agon-ez80asm) - MIT License reference
- `minzc/pkg/z80asm/ez80_instructions.md` - Extracted opcode reference
