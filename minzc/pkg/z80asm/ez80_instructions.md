# eZ80 Instruction Reference for MZA

Extracted from [agon-ez80asm](https://github.com/AgonPlatform/agon-ez80asm) (MIT License)

## ADL Mode Suffix Prefixes

When in ADL mode (24-bit), these prefixes control instruction/operand size:

| Suffix | Code | Meaning |
|--------|------|---------|
| `.SIS` | 0x40 | Short Instruction, Short operands (Z80 compat) |
| `.LIS` | 0x49 | Long Instruction, Short operands |
| `.SIL` | 0x52 | Short Instruction, Long operands |
| `.LIL` | 0x5B | Long Instruction, Long operands (native ADL) |

Suffix `.S` = SIS or SIL (depends on ADL mode)
Suffix `.L` = LIS or LIL (depends on ADL mode)

## eZ80-Only Instructions

### LEA - Load Effective Address
Computes IX+d or IY+d and stores in register pair.

```
LEA BC, IX+d    ; ED 02 dd    (TRANSFORM_P for BC/DE/HL)
LEA DE, IX+d    ; ED 12 dd
LEA HL, IX+d    ; ED 22 dd
LEA IX, IX+d    ; ED 32 dd
LEA IY, IX+d    ; ED 55 dd
LEA IX, IY+d    ; ED 54 dd
LEA IY, IY+d    ; ED 33 dd
LEA BC, IY+d    ; ED 03 dd
LEA DE, IY+d    ; ED 13 dd
LEA HL, IY+d    ; ED 23 dd
```

### PEA - Push Effective Address
Pushes IX+d or IY+d onto stack.

```
PEA IX+d        ; ED 65 dd
PEA IY+d        ; ED 66 dd
```

### MLT - Multiply
8x8 bit multiply, result in register pair.

```
MLT BC          ; ED 4C    (BC = B * C)
MLT DE          ; ED 5C    (DE = D * E)
MLT HL          ; ED 6C    (HL = H * L)
MLT SP          ; ED 7C    (SP = SPH * SPL) - ADL suffix OK
```

## eZ80 Block I/O Instructions

### Input Block Instructions
```
IND2            ; ED 8C    ; (HL) <- (BC), B--, HL--, DE--
IND2R           ; ED 9C    ; repeat IND2 until B=0
INDM            ; ED 8A    ; Z180/eZ80: IN (HL), (C); DEC HL
INDMR           ; ED 9A    ; repeat INDM
INDRX           ; ED CA    ; IN (HL), (C); DEC HL (different from INDR)
INI2            ; ED 84    ; (HL) <- (BC), B--, HL++, DE++
INI2R           ; ED 94    ; repeat INI2 until B=0
INIM            ; ED 82    ; Z180/eZ80 variant
INIMR           ; ED 92    ; repeat INIM
INIRX           ; ED C2    ; eZ80 variant
```

### Output Block Instructions
```
OTD2R           ; ED BC    ; repeat output/dec
OTDM            ; ED 8B    ; Z180/eZ80 variant
OTDMR           ; ED 9B    ; repeat OTDM
OTDRX           ; ED CB    ; eZ80 variant
OTI2R           ; ED B4    ; repeat output/inc
OTIM            ; ED 83    ; Z180/eZ80 variant
OTIMR           ; ED 93    ; repeat OTIM
OTIRX           ; ED C3    ; eZ80 variant
OUTD2           ; ED AC    ; eZ80 single output/dec
OUTI2           ; ED A4    ; eZ80 single output/inc
```

### Extended I/O
```
IN0 r, (n)      ; ED 00+r*8 nn  ; Z180/eZ80: input from port n
OUT0 (n), r     ; ED 01+r*8 nn  ; Z180/eZ80: output to port n
```

## eZ80 Control Instructions

```
SLP             ; ED 76    ; Sleep (low power mode)
STMIX           ; ED 7D    ; Set mixed ADL mode
RSMIX           ; ED 7E    ; Reset mixed ADL mode
```

## Implementation Notes for MZA

### Adding to instruction_table.go

```go
// ADL suffix codes
const (
    CODE_SIS = 0x40
    CODE_LIS = 0x49
    CODE_SIL = 0x52
    CODE_LIL = 0x5B
)

// LEA instructions
var leaInstructions = []InstructionPattern{
    {Mnemonic: "LEA", Operands: []OperandPattern{
        {OpTypeReg16, "BC"}, {OpTypeIndIdx, "IX"}},
        EncodingFunc: encodeLEA, Encoding: []byte{0xED, 0x02}},
    {Mnemonic: "LEA", Operands: []OperandPattern{
        {OpTypeReg16, "DE"}, {OpTypeIndIdx, "IX"}},
        EncodingFunc: encodeLEA, Encoding: []byte{0xED, 0x12}},
    {Mnemonic: "LEA", Operands: []OperandPattern{
        {OpTypeReg16, "HL"}, {OpTypeIndIdx, "IX"}},
        EncodingFunc: encodeLEA, Encoding: []byte{0xED, 0x22}},
    // ... more variants
}

// PEA instructions
var peaInstructions = []InstructionPattern{
    {Mnemonic: "PEA", Operands: []OperandPattern{
        {OpTypeIndIdx, "IX"}},
        EncodingFunc: encodePEA, Encoding: []byte{0xED, 0x65}},
    {Mnemonic: "PEA", Operands: []OperandPattern{
        {OpTypeIndIdx, "IY"}},
        EncodingFunc: encodePEA, Encoding: []byte{0xED, 0x66}},
}

// MLT instructions
var mltInstructions = []InstructionPattern{
    {Mnemonic: "MLT", Operands: []OperandPattern{
        {OpTypeReg16, "BC"}},
        Encoding: []byte{0xED, 0x4C}},
    {Mnemonic: "MLT", Operands: []OperandPattern{
        {OpTypeReg16, "DE"}},
        Encoding: []byte{0xED, 0x5C}},
    {Mnemonic: "MLT", Operands: []OperandPattern{
        {OpTypeReg16, "HL"}},
        Encoding: []byte{0xED, 0x6C}},
    {Mnemonic: "MLT", Operands: []OperandPattern{
        {OpTypeReg16, "SP"}},
        Encoding: []byte{0xED, 0x7C}},
}
```

### Key Differences from Z80

1. **24-bit immediates**: `LD HL, mmnn` loads 3 bytes instead of 2
2. **24-bit addresses**: `JP mmnn` uses 3-byte address
3. **ADL prefix**: Instructions can have `.S` or `.L` suffix
4. **New registers**: SPL (low 16 bits of 24-bit SP)
5. **Memory addressing**: Full 16MB address space

### CPU Mode Detection

```go
type CPUMode int

const (
    CPUModeZ80    CPUMode = iota // Classic Z80 (16-bit)
    CPUModeEZ80Z                  // eZ80 in Z80 mode (ADL=0)
    CPUModeEZ80ADL               // eZ80 in ADL mode (ADL=1)
)

func (a *Assembler) SetCPUMode(mode CPUMode) {
    a.cpuMode = mode
    if mode == CPUModeEZ80ADL {
        a.addressSize = 3  // 24-bit
        a.immediateSize = 3
    } else {
        a.addressSize = 2  // 16-bit
        a.immediateSize = 2
    }
}
```

## References

- [eZ80 CPU User Manual](https://www.zilog.com/docs/um0077.pdf)
- [agon-ez80asm source](https://github.com/AgonPlatform/agon-ez80asm)
- [Agon Light Documentation](https://github.com/breakintoprogram/agon-docs)
