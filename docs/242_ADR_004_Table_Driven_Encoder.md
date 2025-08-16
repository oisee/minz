# ADR-004: Table-Driven Instruction Encoder

## Status
**ACCEPTED** - Implementation in progress

## Context

### Current Problem
The hand-coded encoder in `encoder.go` has massive gaps:
- **226 LD instruction failures** in corpus (90% of all failures)
- Only handles basic LD patterns, missing memory operations
- Ad-hoc implementation makes it hard to add new instructions
- No systematic coverage of Z80 instruction set

### Corpus Analysis
```
LD instructions: 226 failures (90%)
JP/JR instructions: 22 failures (8%)
Arithmetic: 14 failures (5%)
Others: <2% each
```

### Success Rate Impact
Current **12%** could improve to **40-60%** by fixing LD instructions alone!

## Decision

### Implement Comprehensive Table-Driven Encoder
Replace ad-hoc encoding with data-driven instruction tables that:
1. **Cover ALL Z80 instructions** systematically
2. **Handle ALL addressing modes** correctly
3. **Generate optimal encodings** automatically
4. **Easy to extend** with new patterns

## Implementation

### 1. Instruction Table Structure
```go
type InstructionPattern struct {
    Mnemonic     string
    Operands     []OperandPattern
    Encoding     []byte
    EncodingFunc EncodingGenerator
    Cycles       int
    Size         int
    Flags        InstructionFlags
}

type OperandPattern struct {
    Type        OperandType
    Constraint  string // "A", "HL", "nn", "(nn)", etc.
    Position    int    // Where in encoding
    Size        int    // 0, 8, or 16 bits
}

type OperandType int
const (
    OpNone OperandType = iota
    OpReg8             // A, B, C, D, E, H, L
    OpReg16            // BC, DE, HL, SP, IX, IY
    OpImm8             // n
    OpImm16            // nn
    OpIndReg           // (HL), (BC), (DE), (IX+d), (IY+d)
    OpIndImm           // (nn)
    OpRelative         // e (for JR)
)
```

### 2. LD Instruction Complete Table
```go
var ldInstructions = []InstructionPattern{
    // LD r, r' - 8-bit register to register
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg8, "A"}, {OpReg8, "B"}}, Encoding: []byte{0x78}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg8, "A"}, {OpReg8, "C"}}, Encoding: []byte{0x79}},
    // ... all 64 combinations
    
    // LD r, n - 8-bit immediate
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg8, "A"}, {OpImm8, "n"}}, Encoding: []byte{0x3E, 0}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg8, "B"}, {OpImm8, "n"}}, Encoding: []byte{0x06, 0}},
    // ... all registers
    
    // LD r, (HL) - register from memory
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg8, "A"}, {OpIndReg, "(HL)"}}, Encoding: []byte{0x7E}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg8, "B"}, {OpIndReg, "(HL)"}}, Encoding: []byte{0x46}},
    // ... all registers
    
    // LD (HL), r - memory from register
    {Mnemonic: "LD", Operands: []OperandPattern{{OpIndReg, "(HL)"}, {OpReg8, "A"}}, Encoding: []byte{0x77}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpIndReg, "(HL)"}, {OpReg8, "B"}}, Encoding: []byte{0x70}},
    // ... all registers
    
    // LD A, (BC/DE) - accumulator from pointer
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg8, "A"}, {OpIndReg, "(BC)"}}, Encoding: []byte{0x0A}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg8, "A"}, {OpIndReg, "(DE)"}}, Encoding: []byte{0x1A}},
    
    // LD (BC/DE), A - pointer from accumulator
    {Mnemonic: "LD", Operands: []OperandPattern{{OpIndReg, "(BC)"}, {OpReg8, "A"}}, Encoding: []byte{0x02}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpIndReg, "(DE)"}, {OpReg8, "A"}}, Encoding: []byte{0x12}},
    
    // LD A, (nn) - accumulator from address
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg8, "A"}, {OpIndImm, "(nn)"}}, 
     EncodingFunc: encodeLDMemDirect, Encoding: []byte{0x3A}},
    
    // LD (nn), A - address from accumulator  
    {Mnemonic: "LD", Operands: []OperandPattern{{OpIndImm, "(nn)"}, {OpReg8, "A"}}, 
     EncodingFunc: encodeLDMemDirect, Encoding: []byte{0x32}},
    
    // LD HL, (nn) - 16-bit from memory
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg16, "HL"}, {OpIndImm, "(nn)"}}, 
     EncodingFunc: encodeLD16MemDirect, Encoding: []byte{0x2A}},
    
    // LD (nn), HL - memory from 16-bit
    {Mnemonic: "LD", Operands: []OperandPattern{{OpIndImm, "(nn)"}, {OpReg16, "HL"}}, 
     EncodingFunc: encodeLD16MemDirect, Encoding: []byte{0x22}},
    
    // LD rr, nn - 16-bit immediate
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg16, "BC"}, {OpImm16, "nn"}}, 
     EncodingFunc: encodeLD16Imm, Encoding: []byte{0x01}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg16, "DE"}, {OpImm16, "nn"}}, 
     EncodingFunc: encodeLD16Imm, Encoding: []byte{0x11}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg16, "HL"}, {OpImm16, "nn"}}, 
     EncodingFunc: encodeLD16Imm, Encoding: []byte{0x21}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg16, "SP"}, {OpImm16, "nn"}}, 
     EncodingFunc: encodeLD16Imm, Encoding: []byte{0x31}},
    
    // LD SP, HL - special case
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg16, "SP"}, {OpReg16, "HL"}}, 
     Encoding: []byte{0xF9}},
    
    // IX/IY variants (with 0xDD/0xFD prefixes)
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg16, "IX"}, {OpImm16, "nn"}}, 
     EncodingFunc: encodeLD16Imm, Encoding: []byte{0xDD, 0x21}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg16, "IY"}, {OpImm16, "nn"}}, 
     EncodingFunc: encodeLD16Imm, Encoding: []byte{0xFD, 0x21}},
    
    // ED-prefixed 16-bit memory operations
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg16, "BC"}, {OpIndImm, "(nn)"}}, 
     EncodingFunc: encodeLD16MemED, Encoding: []byte{0xED, 0x4B}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg16, "DE"}, {OpIndImm, "(nn)"}}, 
     EncodingFunc: encodeLD16MemED, Encoding: []byte{0xED, 0x5B}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpReg16, "SP"}, {OpIndImm, "(nn)"}}, 
     EncodingFunc: encodeLD16MemED, Encoding: []byte{0xED, 0x7B}},
    
    {Mnemonic: "LD", Operands: []OperandPattern{{OpIndImm, "(nn)"}, {OpReg16, "BC"}}, 
     EncodingFunc: encodeLD16MemED, Encoding: []byte{0xED, 0x43}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpIndImm, "(nn)"}, {OpReg16, "DE"}}, 
     EncodingFunc: encodeLD16MemED, Encoding: []byte{0xED, 0x53}},
    {Mnemonic: "LD", Operands: []OperandPattern{{OpIndImm, "(nn)"}, {OpReg16, "SP"}}, 
     EncodingFunc: encodeLD16MemED, Encoding: []byte{0xED, 0x73}},
}
```

### 3. Pattern Matching Engine
```go
func (a *Assembler) encodeInstruction(line *Line) ([]byte, error) {
    // Look up instruction in table
    patterns := getInstructionPatterns(line.Mnemonic)
    
    for _, pattern := range patterns {
        if match, values := matchPattern(pattern, line); match {
            if pattern.EncodingFunc != nil {
                return pattern.EncodingFunc(a, pattern, values)
            }
            return generateEncoding(pattern, values), nil
        }
    }
    
    return nil, fmt.Errorf("no matching pattern for %s", formatInstruction(line))
}

func matchPattern(pattern InstructionPattern, line *Line) (bool, []interface{}) {
    if len(pattern.Operands) != len(line.Operands) {
        return false, nil
    }
    
    values := make([]interface{}, len(line.Operands))
    
    for i, opPattern := range pattern.Operands {
        operand := line.Operands[i]
        
        switch opPattern.Type {
        case OpReg8:
            if reg := parseReg8(operand); reg != nil {
                if opPattern.Constraint != "" && reg.Name != opPattern.Constraint {
                    return false, nil
                }
                values[i] = reg
            } else {
                return false, nil
            }
            
        case OpReg16:
            if reg := parseReg16(operand); reg != nil {
                if opPattern.Constraint != "" && reg.Name != opPattern.Constraint {
                    return false, nil
                }
                values[i] = reg
            } else {
                return false, nil
            }
            
        case OpImm8, OpImm16:
            if val := parseImmediate(operand); val != nil {
                values[i] = val
            } else {
                return false, nil
            }
            
        case OpIndReg:
            if indirect := parseIndirectReg(operand); indirect != nil {
                if opPattern.Constraint != "" && indirect != opPattern.Constraint {
                    return false, nil
                }
                values[i] = indirect
            } else {
                return false, nil
            }
            
        case OpIndImm:
            if addr := parseIndirectAddr(operand); addr != nil {
                values[i] = addr
            } else {
                return false, nil
            }
        }
    }
    
    return true, values
}
```

### 4. Encoding Generators
```go
func encodeLDMemDirect(a *Assembler, pattern InstructionPattern, values []interface{}) ([]byte, error) {
    // Pattern encoding has base opcode
    result := make([]byte, len(pattern.Encoding))
    copy(result, pattern.Encoding)
    
    // Find the address value
    var addr uint16
    for _, v := range values {
        if a, ok := v.(uint16); ok {
            addr = a
            break
        }
    }
    
    // Add address bytes (little-endian)
    result = append(result, byte(addr), byte(addr>>8))
    return result, nil
}

func encodeLD16Imm(a *Assembler, pattern InstructionPattern, values []interface{}) ([]byte, error) {
    result := make([]byte, len(pattern.Encoding))
    copy(result, pattern.Encoding)
    
    // Find immediate value
    var imm uint16
    for _, v := range values {
        if val, ok := v.(uint16); ok {
            imm = val
            break
        }
    }
    
    // Add immediate bytes (little-endian)
    result = append(result, byte(imm), byte(imm>>8))
    return result, nil
}
```

### 5. Operand Parsing
```go
func parseOperand(operand string) (OperandType, interface{}) {
    operand = strings.TrimSpace(operand)
    
    // Check for indirect addressing
    if strings.HasPrefix(operand, "(") && strings.HasSuffix(operand, ")") {
        inner := operand[1:len(operand)-1]
        
        // Check for register indirect
        if reg := parseReg16(inner); reg != nil {
            return OpIndReg, reg
        }
        
        // Check for address
        if addr := parseAddress(inner); addr != nil {
            return OpIndImm, addr
        }
    }
    
    // Check for 8-bit register
    if reg := parseReg8(operand); reg != nil {
        return OpReg8, reg
    }
    
    // Check for 16-bit register
    if reg := parseReg16(operand); reg != nil {
        return OpReg16, reg
    }
    
    // Check for immediate value
    if val := parseImmediate(operand); val != nil {
        if val <= 0xFF {
            return OpImm8, val
        }
        return OpImm16, val
    }
    
    return OpNone, nil
}
```

## Expected Outcomes

### Before (Current - 12% success)
```asm
; These all fail:
LD HL, ($8000)     ; Error: invalid operands for LD
LD ($8000), HL     ; Error: invalid operands for LD  
LD A, ($SCREEN)    ; Error: invalid operands for LD
LD BC, ($DATA)     ; Error: invalid operands for LD
```

### After (Table-driven - 40-60% success)
```asm
; These all work:
LD HL, ($8000)     ; → 2A 00 80
LD ($8000), HL     ; → 22 00 80
LD A, ($SCREEN)    ; → 3A [addr]
LD BC, ($DATA)     ; → ED 4B [addr]
```

## Benefits

### Immediate Impact
- **+30-40% success rate** - Fix 226 LD failures
- **Systematic coverage** - All Z80 instructions
- **Maintainable** - Data-driven, not code-driven
- **Extensible** - Easy to add new patterns

### Long-term Benefits  
- **Complete Z80 support** - Professional assembler
- **Optimization potential** - Choose best encoding
- **Documentation** - Table IS the documentation
- **Verification** - Can validate against Z80 specs

## Implementation Plan

### Phase 1: Core LD Instructions (1 day)
- [ ] Create instruction table structure
- [ ] Implement all LD patterns
- [ ] Build pattern matching engine
- [ ] Test against corpus

### Phase 2: Control Flow (0.5 day)
- [ ] JP/JR/CALL/RET instructions
- [ ] Conditional variants
- [ ] Relative addressing

### Phase 3: Arithmetic/Logic (0.5 day)
- [ ] ADD/ADC/SUB/SBC/CP
- [ ] AND/OR/XOR
- [ ] INC/DEC
- [ ] Shift/Rotate

### Phase 4: Complete Set (1 day)
- [ ] Stack operations
- [ ] Bit operations
- [ ] Block operations
- [ ] Special instructions

## Success Metrics

### Quantitative
- **Success rate**: 12% → 40-60%
- **LD coverage**: 100% of standard patterns
- **Instruction coverage**: 95%+ of Z80 set
- **Performance**: <1ms per instruction

### Qualitative
- **Code quality**: Clean, maintainable tables
- **Documentation**: Self-documenting patterns
- **Extensibility**: Easy to add instructions
- **Correctness**: Matches Z80 specifications

This is THE critical improvement that will unlock MinZ compilation! 🚀