package z80asm

import (
	"fmt"
	"strings"
)

// eZ80-specific instructions
// Reference: agon-ez80asm (MIT License)
// https://github.com/AgonPlatform/agon-ez80asm

// ez80Instructions contains eZ80-only instruction patterns
var ez80Instructions = []InstructionPattern{
	// ========== LEA - Load Effective Address ==========
	// LEA rr, IX+d / LEA rr, IY+d
	{Mnemonic: "LEA", Operands: []OperandPattern{
		{OpTypeReg16, "BC"}, {OpTypeIdxDisp, ""}},
		EncodingFunc: encodeLEA, Encoding: []byte{0xED, 0x02}},
	{Mnemonic: "LEA", Operands: []OperandPattern{
		{OpTypeReg16, "DE"}, {OpTypeIdxDisp, ""}},
		EncodingFunc: encodeLEA, Encoding: []byte{0xED, 0x12}},
	{Mnemonic: "LEA", Operands: []OperandPattern{
		{OpTypeReg16, "HL"}, {OpTypeIdxDisp, ""}},
		EncodingFunc: encodeLEA, Encoding: []byte{0xED, 0x22}},
	{Mnemonic: "LEA", Operands: []OperandPattern{
		{OpTypeReg16, "IX"}, {OpTypeIdxDisp, ""}},
		EncodingFunc: encodeLEA, Encoding: []byte{0xED, 0x32}},
	{Mnemonic: "LEA", Operands: []OperandPattern{
		{OpTypeReg16, "IY"}, {OpTypeIdxDisp, ""}},
		EncodingFunc: encodeLEA, Encoding: []byte{0xED, 0x33}},

	// ========== PEA - Push Effective Address ==========
	{Mnemonic: "PEA", Operands: []OperandPattern{
		{OpTypeIdxDisp, ""}},
		EncodingFunc: encodePEA, Encoding: []byte{0xED, 0x65}},

	// ========== MLT - 8x8 Multiply ==========
	// MLT rr: result = high_byte * low_byte
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

	// ========== Control Instructions ==========
	{Mnemonic: "SLP", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x76}}, // Sleep/low power mode
	{Mnemonic: "STMIX", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x7D}}, // Set mixed ADL mode
	{Mnemonic: "RSMIX", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x7E}}, // Reset mixed ADL mode

	// ========== Extended Block Input ==========
	{Mnemonic: "IND2", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x8C}},
	{Mnemonic: "IND2R", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x9C}},
	{Mnemonic: "INDM", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x8A}},
	{Mnemonic: "INDMR", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x9A}},
	{Mnemonic: "INDRX", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0xCA}},
	{Mnemonic: "INI2", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x84}},
	{Mnemonic: "INI2R", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x94}},
	{Mnemonic: "INIM", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x82}},
	{Mnemonic: "INIMR", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x92}},
	{Mnemonic: "INIRX", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0xC2}},

	// ========== Extended Block Output ==========
	{Mnemonic: "OTD2R", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0xBC}},
	{Mnemonic: "OTDM", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x8B}},
	{Mnemonic: "OTDMR", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x9B}},
	{Mnemonic: "OTDRX", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0xCB}},
	{Mnemonic: "OTI2R", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0xB4}},
	{Mnemonic: "OTIM", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x83}},
	{Mnemonic: "OTIMR", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0x93}},
	{Mnemonic: "OTIRX", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0xC3}},
	{Mnemonic: "OUTD2", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0xAC}},
	{Mnemonic: "OUTI2", Operands: []OperandPattern{},
		Encoding: []byte{0xED, 0xA4}},

	// ========== IN0/OUT0 - Port 0 I/O (Z180/eZ80) ==========
	// IN0 r, (n)
	{Mnemonic: "IN0", Operands: []OperandPattern{
		{OpTypeReg8, "A"}, {OpTypeIndImm, ""}},
		EncodingFunc: encodeIN0, Encoding: []byte{0xED, 0x38}},
	{Mnemonic: "IN0", Operands: []OperandPattern{
		{OpTypeReg8, "B"}, {OpTypeIndImm, ""}},
		EncodingFunc: encodeIN0, Encoding: []byte{0xED, 0x00}},
	{Mnemonic: "IN0", Operands: []OperandPattern{
		{OpTypeReg8, "C"}, {OpTypeIndImm, ""}},
		EncodingFunc: encodeIN0, Encoding: []byte{0xED, 0x08}},
	{Mnemonic: "IN0", Operands: []OperandPattern{
		{OpTypeReg8, "D"}, {OpTypeIndImm, ""}},
		EncodingFunc: encodeIN0, Encoding: []byte{0xED, 0x10}},
	{Mnemonic: "IN0", Operands: []OperandPattern{
		{OpTypeReg8, "E"}, {OpTypeIndImm, ""}},
		EncodingFunc: encodeIN0, Encoding: []byte{0xED, 0x18}},
	{Mnemonic: "IN0", Operands: []OperandPattern{
		{OpTypeReg8, "H"}, {OpTypeIndImm, ""}},
		EncodingFunc: encodeIN0, Encoding: []byte{0xED, 0x20}},
	{Mnemonic: "IN0", Operands: []OperandPattern{
		{OpTypeReg8, "L"}, {OpTypeIndImm, ""}},
		EncodingFunc: encodeIN0, Encoding: []byte{0xED, 0x28}},

	// OUT0 (n), r
	{Mnemonic: "OUT0", Operands: []OperandPattern{
		{OpTypeIndImm, ""}, {OpTypeReg8, "A"}},
		EncodingFunc: encodeOUT0, Encoding: []byte{0xED, 0x39}},
	{Mnemonic: "OUT0", Operands: []OperandPattern{
		{OpTypeIndImm, ""}, {OpTypeReg8, "B"}},
		EncodingFunc: encodeOUT0, Encoding: []byte{0xED, 0x01}},
	{Mnemonic: "OUT0", Operands: []OperandPattern{
		{OpTypeIndImm, ""}, {OpTypeReg8, "C"}},
		EncodingFunc: encodeOUT0, Encoding: []byte{0xED, 0x09}},
	{Mnemonic: "OUT0", Operands: []OperandPattern{
		{OpTypeIndImm, ""}, {OpTypeReg8, "D"}},
		EncodingFunc: encodeOUT0, Encoding: []byte{0xED, 0x11}},
	{Mnemonic: "OUT0", Operands: []OperandPattern{
		{OpTypeIndImm, ""}, {OpTypeReg8, "E"}},
		EncodingFunc: encodeOUT0, Encoding: []byte{0xED, 0x19}},
	{Mnemonic: "OUT0", Operands: []OperandPattern{
		{OpTypeIndImm, ""}, {OpTypeReg8, "H"}},
		EncodingFunc: encodeOUT0, Encoding: []byte{0xED, 0x21}},
	{Mnemonic: "OUT0", Operands: []OperandPattern{
		{OpTypeIndImm, ""}, {OpTypeReg8, "L"}},
		EncodingFunc: encodeOUT0, Encoding: []byte{0xED, 0x29}},

	// ========== TST - Test (AND without storing result) ==========
	{Mnemonic: "TST", Operands: []OperandPattern{
		{OpTypeReg8, "A"}, {OpTypeReg8, ""}},
		EncodingFunc: encodeTST, Encoding: []byte{0xED, 0x04}},
	{Mnemonic: "TST", Operands: []OperandPattern{
		{OpTypeReg8, "A"}, {OpTypeImm8, ""}},
		EncodingFunc: encodeTSTImm, Encoding: []byte{0xED, 0x64}},
}

// Encoding functions for eZ80 instructions

// encodeLEA encodes LEA rr, IX+d or LEA rr, IY+d
// The base encoding in p.Encoding is for IX variants.
// IY variants use +1 on the second opcode byte (BC: 02→03, DE: 12→13, etc.)
// Special cases: LEA IY,IX+d = ED 55, LEA IX,IY+d = ED 54, LEA IY,IY+d = ED 33
func encodeLEA(a *Assembler, p *InstructionPattern, operands []interface{}) ([]byte, error) {
	if len(operands) < 2 {
		return nil, &AssemblerError{Message: "LEA requires two operands"}
	}

	idx, ok := operands[1].(*idxDisp)
	if !ok {
		return nil, &AssemblerError{Message: "LEA second operand must be IX+d or IY+d"}
	}

	dstReg := ""
	if s, ok := operands[0].(string); ok {
		dstReg = strings.ToUpper(s)
	}

	// Determine opcode byte based on destination register and source index register
	var opcode byte
	switch {
	case dstReg == "BC" && idx.Reg == "IX":
		opcode = 0x02
	case dstReg == "BC" && idx.Reg == "IY":
		opcode = 0x03
	case dstReg == "DE" && idx.Reg == "IX":
		opcode = 0x12
	case dstReg == "DE" && idx.Reg == "IY":
		opcode = 0x13
	case dstReg == "HL" && idx.Reg == "IX":
		opcode = 0x22
	case dstReg == "HL" && idx.Reg == "IY":
		opcode = 0x23
	case dstReg == "IX" && idx.Reg == "IX":
		opcode = 0x32
	case dstReg == "IX" && idx.Reg == "IY":
		opcode = 0x54
	case dstReg == "IY" && idx.Reg == "IX":
		opcode = 0x55
	case dstReg == "IY" && idx.Reg == "IY":
		opcode = 0x33
	default:
		return nil, &AssemblerError{Message: fmt.Sprintf("invalid LEA combination: %s, %s+d", dstReg, idx.Reg)}
	}

	return []byte{0xED, opcode, byte(idx.Disp)}, nil
}

// encodePEA encodes PEA IX+d or PEA IY+d
// PEA IX+d = ED 65 dd, PEA IY+d = ED 66 dd
func encodePEA(a *Assembler, p *InstructionPattern, operands []interface{}) ([]byte, error) {
	if len(operands) < 1 {
		return nil, &AssemblerError{Message: "PEA requires an operand"}
	}

	idx, ok := operands[0].(*idxDisp)
	if !ok {
		return nil, &AssemblerError{Message: "PEA operand must be IX+d or IY+d"}
	}

	opcode := byte(0x65) // IX
	if idx.Reg == "IY" {
		opcode = 0x66
	}

	return []byte{0xED, opcode, byte(idx.Disp)}, nil
}

// encodeIN0 encodes IN0 r, (n)
func encodeIN0(a *Assembler, p *InstructionPattern, operands []interface{}) ([]byte, error) {
	port := byte(0)
	if len(operands) > 1 {
		if n, ok := operands[1].(int); ok {
			if n < 0 || n > 255 {
				return nil, &AssemblerError{Message: "port number out of range (0-255)"}
			}
			port = byte(n)
		}
	}

	result := make([]byte, len(p.Encoding)+1)
	copy(result, p.Encoding)
	result[len(p.Encoding)] = port

	return result, nil
}

// encodeOUT0 encodes OUT0 (n), r
func encodeOUT0(a *Assembler, p *InstructionPattern, operands []interface{}) ([]byte, error) {
	port := byte(0)
	if len(operands) > 0 {
		if n, ok := operands[0].(int); ok {
			if n < 0 || n > 255 {
				return nil, &AssemblerError{Message: "port number out of range (0-255)"}
			}
			port = byte(n)
		}
	}

	result := make([]byte, len(p.Encoding)+1)
	copy(result, p.Encoding)
	result[len(p.Encoding)] = port

	return result, nil
}

// encodeTST encodes TST A, r
// Encoding: ED (04 + r*8) where r = B:0 C:1 D:2 E:3 H:4 L:5 (HL):6 A:7
func encodeTST(a *Assembler, p *InstructionPattern, operands []interface{}) ([]byte, error) {
	regIndex := byte(0)
	if len(operands) > 1 {
		if name, ok := operands[1].(string); ok {
			regIndex = reg8Index(name)
		}
	}

	result := make([]byte, len(p.Encoding))
	copy(result, p.Encoding)
	result[1] |= (regIndex << 3)

	return result, nil
}

// reg8Index returns the Z80 3-bit register index: B=0 C=1 D=2 E=3 H=4 L=5 (HL)=6 A=7
func reg8Index(name string) byte {
	switch strings.ToUpper(name) {
	case "B":
		return 0
	case "C":
		return 1
	case "D":
		return 2
	case "E":
		return 3
	case "H":
		return 4
	case "L":
		return 5
	case "(HL)":
		return 6
	case "A":
		return 7
	}
	return 0
}

// encodeTSTImm encodes TST A, n
func encodeTSTImm(a *Assembler, p *InstructionPattern, operands []interface{}) ([]byte, error) {
	imm := byte(0)
	if len(operands) > 1 {
		if n, ok := operands[1].(int); ok {
			imm = byte(n)
		}
	}

	result := make([]byte, len(p.Encoding)+1)
	copy(result, p.Encoding)
	result[len(p.Encoding)] = imm

	return result, nil
}

// init adds eZ80 instructions to the global table
func init() {
	instructionTable = append(instructionTable, ez80Instructions...)
}
