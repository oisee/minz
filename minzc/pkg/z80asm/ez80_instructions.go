package z80asm

// eZ80-specific instructions
// Reference: agon-ez80asm (MIT License)
// https://github.com/AgonPlatform/agon-ez80asm

// ez80Instructions contains eZ80-only instruction patterns
var ez80Instructions = []InstructionPattern{
	// ========== LEA - Load Effective Address ==========
	// LEA rr, IX+d
	{Mnemonic: "LEA", Operands: []OperandPattern{
		{OpTypeReg16, "BC"}, {OpTypeIndIdx, ""}},
		EncodingFunc: encodeLEA_IX, Encoding: []byte{0xED, 0x02}},
	{Mnemonic: "LEA", Operands: []OperandPattern{
		{OpTypeReg16, "DE"}, {OpTypeIndIdx, ""}},
		EncodingFunc: encodeLEA_IX, Encoding: []byte{0xED, 0x12}},
	{Mnemonic: "LEA", Operands: []OperandPattern{
		{OpTypeReg16, "HL"}, {OpTypeIndIdx, ""}},
		EncodingFunc: encodeLEA_IX, Encoding: []byte{0xED, 0x22}},
	{Mnemonic: "LEA", Operands: []OperandPattern{
		{OpTypeReg16, "IX"}, {OpTypeIndIdx, ""}},
		EncodingFunc: encodeLEA_IX, Encoding: []byte{0xED, 0x32}},
	{Mnemonic: "LEA", Operands: []OperandPattern{
		{OpTypeReg16, "IY"}, {OpTypeIndIdx, ""}},
		EncodingFunc: encodeLEA_IY_from_IX, Encoding: []byte{0xED, 0x55}},

	// ========== PEA - Push Effective Address ==========
	{Mnemonic: "PEA", Operands: []OperandPattern{
		{OpTypeIndIdx, ""}},
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

// encodeLEA_IX encodes LEA rr, IX+d
func encodeLEA_IX(a *Assembler, p *InstructionPattern, operands []interface{}) ([]byte, error) {
	// Operand 1 is destination register (determines opcode via pattern)
	// Operand 2 is (IX+d) - we need to extract displacement

	disp := int8(0)
	if len(operands) > 1 {
		if d, ok := operands[1].(int); ok {
			if d < -128 || d > 127 {
				return nil, &AssemblerError{Message: "displacement out of range (-128 to 127)"}
			}
			disp = int8(d)
		}
	}

	// Check if it's IX or IY based on the operand string
	// For now, assume IX (0xDD prefix would be needed for IY)
	result := make([]byte, len(p.Encoding)+1)
	copy(result, p.Encoding)
	result[len(p.Encoding)] = byte(disp)

	return result, nil
}

// encodeLEA_IY_from_IX encodes LEA IY, IX+d (special case)
func encodeLEA_IY_from_IX(a *Assembler, p *InstructionPattern, operands []interface{}) ([]byte, error) {
	disp := int8(0)
	if len(operands) > 1 {
		if d, ok := operands[1].(int); ok {
			if d < -128 || d > 127 {
				return nil, &AssemblerError{Message: "displacement out of range (-128 to 127)"}
			}
			disp = int8(d)
		}
	}

	result := make([]byte, len(p.Encoding)+1)
	copy(result, p.Encoding)
	result[len(p.Encoding)] = byte(disp)

	return result, nil
}

// encodePEA encodes PEA IX+d or PEA IY+d
func encodePEA(a *Assembler, p *InstructionPattern, operands []interface{}) ([]byte, error) {
	disp := int8(0)
	isIY := false

	if len(operands) > 0 {
		// Check if it's IY (would need different opcode)
		if str, ok := operands[0].(string); ok {
			if len(str) > 0 && (str[0] == 'Y' || str[0] == 'y') {
				isIY = true
			}
		}
		if d, ok := operands[0].(int); ok {
			if d < -128 || d > 127 {
				return nil, &AssemblerError{Message: "displacement out of range (-128 to 127)"}
			}
			disp = int8(d)
		}
	}

	result := make([]byte, len(p.Encoding)+1)
	copy(result, p.Encoding)
	if isIY {
		result[1] = 0x66 // PEA IY+d
	}
	result[len(p.Encoding)] = byte(disp)

	return result, nil
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
func encodeTST(a *Assembler, p *InstructionPattern, operands []interface{}) ([]byte, error) {
	// Get register index for operand 2
	regIndex := byte(0)
	if len(operands) > 1 {
		if idx, ok := operands[1].(int); ok {
			regIndex = byte(idx)
		}
	}

	result := make([]byte, len(p.Encoding))
	copy(result, p.Encoding)
	result[1] |= (regIndex << 3) // TST A, r: opcode has register in bits 5-3

	return result, nil
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
