package z80asm

import "fmt"

// This file contains the remaining instruction set registration functions

// registerVirtualInstructions registers sjasmplus-style virtual instructions
func registerVirtualInstructions() {
	// LD HL, DE -> LD H, D : LD L, E
	addInstruction("LD", &InstructionDef{
		Mnemonic: "LD",
		Operands: []OperandType{OpReg16, OpReg16},
		Size:     2,
		Encoder:  encodeVirtualLD16,
	})
	
	// LD HL, BC -> LD H, B : LD L, C
	// (Covered by above OpReg16, OpReg16 pattern)
	
	// Multi-operand instructions
	// SRL A, A, A -> SRL A : SRL A : SRL A (3 shifts)
	addInstruction("SRL", &InstructionDef{
		Mnemonic: "SRL",
		Operands: []OperandType{OpReg8, OpReg8, OpReg8}, // Same register 3 times
		Size:     6, // 3 x CB + opcode
		Encoder:  encodeVirtualShiftTriple,
	})
	
	// XOR A, B -> XOR A : XOR B (but this doesn't make sense - XOR A clears A!)
	// Maybe they meant XOR B, C -> XOR B : XOR C?
}

// registerBitInstructions registers bit manipulation instructions
func registerBitInstructions() {
	// Rotate instructions
	rotateOps := []struct {
		mnemonic string
		noReg    byte // No register (accumulator implied)
		cbBase   byte // CB prefix base
	}{
		{"RLCA", 0x07, 0x00}, // RLC for CB
		{"RRCA", 0x0F, 0x08}, // RRC for CB
		{"RLA", 0x17, 0x10},  // RL for CB
		{"RRA", 0x1F, 0x18},  // RR for CB
	}
	
	// Register rotates without prefix (accumulator only)
	for _, op := range rotateOps {
		if op.noReg != 0 {
			addInstruction(op.mnemonic, &InstructionDef{
				Mnemonic: op.mnemonic,
				Operands: []OperandType{},
				Size:     1,
				Encoder:  encodeImplied(op.noReg),
			})
		}
	}
	
	// CB prefix rotates and shifts
	cbRotateOps := []struct {
		mnemonic string
		base     byte
	}{
		{"RLC", 0x00},
		{"RRC", 0x08},
		{"RL", 0x10},
		{"RR", 0x18},
		{"SLA", 0x20},
		{"SRA", 0x28},
		{"SRL", 0x38},
	}
	
	for _, op := range cbRotateOps {
		// Register forms
		for r := byte(0); r < 8; r++ {
			opcode := op.base | r
			
			addInstruction(op.mnemonic, &InstructionDef{
				Mnemonic: op.mnemonic,
				Operands: []OperandType{OpReg8},
				Size:     2,
				Encoder:  makeCBRegEncoder(opcode),
			})
		}
		
		// (HL) form
		addInstruction(op.mnemonic, &InstructionDef{
			Mnemonic: op.mnemonic,
			Operands: []OperandType{OpRegIndirect},
			Size:     2,
			Encoder:  makeCBRegEncoder(op.base | 0x06),
		})
		
		// (IX+d) and (IY+d) forms
		addInstruction(op.mnemonic, &InstructionDef{
			Mnemonic: op.mnemonic,
			Operands: []OperandType{OpIXOffset},
			Size:     4,
			Encoder:  makeCBIndexEncoder(op.base | 0x06, true),
		})
		
		addInstruction(op.mnemonic, &InstructionDef{
			Mnemonic: op.mnemonic,
			Operands: []OperandType{OpIYOffset},
			Size:     4,
			Encoder:  makeCBIndexEncoder(op.base | 0x06, false),
		})
	}
	
	// BIT, RES, SET instructions
	bitOps := []struct {
		mnemonic string
		base     byte
	}{
		{"BIT", 0x40},
		{"RES", 0x80},
		{"SET", 0xC0},
	}
	
	for _, op := range bitOps {
		// BIT/RES/SET b, r
		addInstruction(op.mnemonic, &InstructionDef{
			Mnemonic: op.mnemonic,
			Operands: []OperandType{OpBit, OpReg8},
			Size:     2,
			Encoder:  makeBitOpEncoder(op.base),
		})
		
		// BIT/RES/SET b, (HL)
		addInstruction(op.mnemonic, &InstructionDef{
			Mnemonic: op.mnemonic,
			Operands: []OperandType{OpBit, OpRegIndirect},
			Size:     2,
			Encoder:  makeBitOpEncoder(op.base),
		})
		
		// BIT/RES/SET b, (IX+d)
		addInstruction(op.mnemonic, &InstructionDef{
			Mnemonic: op.mnemonic,
			Operands: []OperandType{OpBit, OpIXOffset},
			Size:     4,
			Encoder:  makeBitIndexEncoder(op.base, true),
		})
		
		// BIT/RES/SET b, (IY+d)
		addInstruction(op.mnemonic, &InstructionDef{
			Mnemonic: op.mnemonic,
			Operands: []OperandType{OpBit, OpIYOffset},
			Size:     4,
			Encoder:  makeBitIndexEncoder(op.base, false),
		})
	}
	
	// RRD and RLD
	addInstruction("RRD", &InstructionDef{
		Mnemonic: "RRD",
		Operands: []OperandType{},
		Size:     2,
		Encoder:  encodeEDPrefix(0x67),
	})
	
	addInstruction("RLD", &InstructionDef{
		Mnemonic: "RLD",
		Operands: []OperandType{},
		Size:     2,
		Encoder:  encodeEDPrefix(0x6F),
	})
}

// registerJumpInstructions registers jump and call instructions
func registerJumpInstructions() {
	// Unconditional jumps
	addInstruction("JP", &InstructionDef{
		Mnemonic: "JP",
		Operands: []OperandType{OpAddr16},
		Size:     3,
		Encoder:  makeJumpEncoder(0xC3, false),
	})
	
	// JP (HL/IX/IY)
	addInstruction("JP", &InstructionDef{
		Mnemonic: "JP",
		Operands: []OperandType{OpRegIndirect},
		Size:     1,
		Encoder:  encodeJPIndirect,
	})
	
	// Conditional jumps
	condJumps := []struct {
		cond   string
		opcode byte
	}{
		{"NZ", 0xC2}, {"Z", 0xCA}, {"NC", 0xD2}, {"C", 0xDA},
		{"PO", 0xE2}, {"PE", 0xEA}, {"P", 0xF2}, {"M", 0xFA},
	}
	
	for _, cj := range condJumps {
		addInstruction("JP", &InstructionDef{
			Mnemonic: "JP",
			Operands: []OperandType{OpCondition, OpAddr16},
			Size:     3,
			Encoder:  makeCondJumpEncoder(cj.opcode),
		})
	}
	
	// Relative jumps
	addInstruction("JR", &InstructionDef{
		Mnemonic: "JR",
		Operands: []OperandType{OpRelative},
		Size:     2,
		Encoder:  makeRelativeEncoder(0x18),
	})
	
	// Conditional relative jumps
	condRelJumps := []struct {
		cond   string
		opcode byte
	}{
		{"NZ", 0x20}, {"Z", 0x28}, {"NC", 0x30}, {"C", 0x38},
	}
	
	for _, cj := range condRelJumps {
		addInstruction("JR", &InstructionDef{
			Mnemonic: "JR",
			Operands: []OperandType{OpCondition, OpRelative},
			Size:     2,
			Encoder:  makeCondRelativeEncoder(cj.opcode),
		})
	}
	
	// DJNZ
	addInstruction("DJNZ", &InstructionDef{
		Mnemonic: "DJNZ",
		Operands: []OperandType{OpRelative},
		Size:     2,
		Encoder:  makeRelativeEncoder(0x10),
	})
	
	// CALL
	addInstruction("CALL", &InstructionDef{
		Mnemonic: "CALL",
		Operands: []OperandType{OpAddr16},
		Size:     3,
		Encoder:  makeJumpEncoder(0xCD, false),
	})
	
	// Conditional CALL
	condCalls := []struct {
		cond   string
		opcode byte
	}{
		{"NZ", 0xC4}, {"Z", 0xCC}, {"NC", 0xD4}, {"C", 0xDC},
		{"PO", 0xE4}, {"PE", 0xEC}, {"P", 0xF4}, {"M", 0xFC},
	}
	
	for _, cc := range condCalls {
		addInstruction("CALL", &InstructionDef{
			Mnemonic: "CALL",
			Operands: []OperandType{OpCondition, OpAddr16},
			Size:     3,
			Encoder:  makeCondJumpEncoder(cc.opcode),
		})
	}
	
	// RET
	addInstruction("RET", &InstructionDef{
		Mnemonic: "RET",
		Operands: []OperandType{},
		Size:     1,
		Encoder:  encodeImplied(0xC9),
	})
	
	// Conditional RET
	condRets := []struct {
		cond   string
		opcode byte
	}{
		{"NZ", 0xC0}, {"Z", 0xC8}, {"NC", 0xD0}, {"C", 0xD8},
		{"PO", 0xE0}, {"PE", 0xE8}, {"P", 0xF0}, {"M", 0xF8},
	}
	
	for _, cr := range condRets {
		addInstruction("RET", &InstructionDef{
			Mnemonic: "RET",
			Operands: []OperandType{OpCondition},
			Size:     1,
			Encoder:  makeCondRetEncoder(cr.opcode),
		})
	}
	
	// RETI and RETN
	addInstruction("RETI", &InstructionDef{
		Mnemonic: "RETI",
		Operands: []OperandType{},
		Size:     2,
		Encoder:  encodeEDPrefix(0x4D),
	})
	
	addInstruction("RETN", &InstructionDef{
		Mnemonic: "RETN",
		Operands: []OperandType{},
		Size:     2,
		Encoder:  encodeEDPrefix(0x45),
	})
	
	// RST - just register once, the encoder handles all vectors
	addInstruction("RST", &InstructionDef{
		Mnemonic: "RST",
		Operands: []OperandType{OpImm8},
		Size:     1,
		Encoder:  makeRSTEncoder(0), // Parameter not used anymore
	})
}

// registerStackInstructions registers stack operations
func registerStackInstructions() {
	// PUSH
	pushRegs := []struct {
		reg    string
		opcode byte
	}{
		{"BC", 0xC5}, {"DE", 0xD5}, {"HL", 0xE5}, {"AF", 0xF5},
	}
	
	for _, pr := range pushRegs {
		addInstruction("PUSH", &InstructionDef{
			Mnemonic: "PUSH",
			Operands: []OperandType{OpReg16},
			Size:     1,
			Encoder:  makeStackEncoder(pr.opcode, true),
		})
	}
	
	// PUSH IX/IY
	addInstruction("PUSH", &InstructionDef{
		Mnemonic: "PUSH",
		Operands: []OperandType{OpReg16},
		Size:     2,
		Encoder:  makeIndexStackEncoder(0xE5, true),
	})
	
	// POP
	popRegs := []struct {
		reg    string
		opcode byte
	}{
		{"BC", 0xC1}, {"DE", 0xD1}, {"HL", 0xE1}, {"AF", 0xF1},
	}
	
	for _, pr := range popRegs {
		addInstruction("POP", &InstructionDef{
			Mnemonic: "POP",
			Operands: []OperandType{OpReg16},
			Size:     1,
			Encoder:  makeStackEncoder(pr.opcode, false),
		})
	}
	
	// POP IX/IY
	addInstruction("POP", &InstructionDef{
		Mnemonic: "POP",
		Operands: []OperandType{OpReg16},
		Size:     2,
		Encoder:  makeIndexStackEncoder(0xE1, false),
	})
}

// registerIOInstructions registers I/O instructions
func registerIOInstructions() {
	// IN A, (n)
	addInstruction("IN", &InstructionDef{
		Mnemonic: "IN",
		Operands: []OperandType{OpReg8, OpAddr16},
		Size:     2,
		Encoder:  encodeINOUT,
	})
	
	// OUT (n), A
	addInstruction("OUT", &InstructionDef{
		Mnemonic: "OUT",
		Operands: []OperandType{OpAddr16, OpReg8},
		Size:     2,
		Encoder:  encodeINOUT,
	})
	
	// IN r, (C) - ED prefix
	inRegs := []struct {
		reg    string
		opcode byte
	}{
		{"B", 0x40}, {"C", 0x48}, {"D", 0x50}, {"E", 0x58},
		{"H", 0x60}, {"L", 0x68}, {"A", 0x78},
	}
	
	for _, ir := range inRegs {
		addInstruction("IN", &InstructionDef{
			Mnemonic: "IN",
			Operands: []OperandType{OpReg8, OpRegIndirect},
			Size:     2,
			Encoder:  makeEDIOEncoder(ir.opcode, true),
		})
	}
	
	// Special: IN F, (C) - affects flags only
	addInstruction("IN", &InstructionDef{
		Mnemonic: "IN",
		Operands: []OperandType{OpReg8, OpRegIndirect},
		Size:     2,
		Encoder:  makeEDIOEncoder(0x70, true),
	})
	
	// OUT (C), r - ED prefix
	outRegs := []struct {
		reg    string
		opcode byte
	}{
		{"B", 0x41}, {"C", 0x49}, {"D", 0x51}, {"E", 0x59},
		{"H", 0x61}, {"L", 0x69}, {"A", 0x79},
	}
	
	for _, or := range outRegs {
		addInstruction("OUT", &InstructionDef{
			Mnemonic: "OUT",
			Operands: []OperandType{OpRegIndirect, OpReg8},
			Size:     2,
			Encoder:  makeEDIOEncoder(or.opcode, false),
		})
	}
	
	// Block I/O instructions
	blockIO := []struct {
		mnemonic string
		opcode   byte
	}{
		{"INI", 0xA2}, {"INIR", 0xB2}, {"IND", 0xAA}, {"INDR", 0xBA},
		{"OUTI", 0xA3}, {"OTIR", 0xB3}, {"OUTD", 0xAB}, {"OTDR", 0xBB},
	}
	
	for _, bio := range blockIO {
		addInstruction(bio.mnemonic, &InstructionDef{
			Mnemonic: bio.mnemonic,
			Operands: []OperandType{},
			Size:     2,
			Encoder:  encodeEDPrefix(bio.opcode),
		})
	}
}

// registerControlInstructions registers control instructions
func registerControlInstructions() {
	// NOP
	addInstruction("NOP", &InstructionDef{
		Mnemonic: "NOP",
		Operands: []OperandType{},
		Size:     1,
		Encoder:  encodeImplied(0x00),
	})
	
	// HALT
	addInstruction("HALT", &InstructionDef{
		Mnemonic: "HALT",
		Operands: []OperandType{},
		Size:     1,
		Encoder:  encodeImplied(0x76),
	})
	
	// DI and EI
	addInstruction("DI", &InstructionDef{
		Mnemonic: "DI",
		Operands: []OperandType{},
		Size:     1,
		Encoder:  encodeImplied(0xF3),
	})
	
	addInstruction("EI", &InstructionDef{
		Mnemonic: "EI",
		Operands: []OperandType{},
		Size:     1,
		Encoder:  encodeImplied(0xFB),
	})
	
	// IM 0/1/2
	imModes := []struct {
		mode   string
		opcode byte
	}{
		{"0", 0x46}, {"1", 0x56}, {"2", 0x5E},
	}
	
	for _, im := range imModes {
		addInstruction("IM", &InstructionDef{
			Mnemonic: "IM",
			Operands: []OperandType{OpImm8},
			Size:     2,
			Encoder:  makeIMEncoder(im.opcode),
		})
	}
	
	// LD I, A and LD R, A
	addInstruction("LD", &InstructionDef{
		Mnemonic: "LD",
		Operands: []OperandType{OpReg8, OpReg8},
		Size:     2,
		Encoder:  encodeSpecialLD,
	})
	
	// SCF and CCF
	addInstruction("SCF", &InstructionDef{
		Mnemonic: "SCF",
		Operands: []OperandType{},
		Size:     1,
		Encoder:  encodeImplied(0x37),
	})
	
	addInstruction("CCF", &InstructionDef{
		Mnemonic: "CCF",
		Operands: []OperandType{},
		Size:     1,
		Encoder:  encodeImplied(0x3F),
	})
}

// Encoder helper functions

func makeCBRegEncoder(opcode byte) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 1 {
			return nil, fmt.Errorf("instruction requires 1 operand")
		}
		
		if isIndirect(line.Operands[0]) && line.Operands[0] == "(HL)" {
			return []byte{0xCB, opcode}, nil
		}
		
		reg, ok := parseRegister(line.Operands[0])
		if !ok {
			return nil, fmt.Errorf("invalid register")
		}
		
		regCode, err := encodeReg8(reg)
		if err != nil {
			return nil, err
		}
		
		// Replace register bits in opcode
		finalOpcode := (opcode & 0xF8) | regCode
		return []byte{0xCB, finalOpcode}, nil
	}
}

func makeCBIndexEncoder(opcode byte, isIX bool) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 1 {
			return nil, fmt.Errorf("instruction requires 1 operand")
		}
		
		offset, err := getIndexOffset(line.Operands[0])
		if err != nil {
			return nil, err
		}
		
		prefix := byte(0xDD)
		if !isIX {
			prefix = 0xFD
		}
		
		return []byte{prefix, 0xCB, byte(offset), opcode}, nil
	}
}

func makeBitOpEncoder(baseOpcode byte) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 2 {
			return nil, fmt.Errorf("instruction requires 2 operands")
		}
		
		// Parse bit number
		bitNum, err := parseOperandValue(line.Operands[0])
		if err != nil {
			return nil, err
		}
		if bitNum > 7 {
			return nil, fmt.Errorf("bit number must be 0-7")
		}
		
		// Parse register/memory operand
		var regCode byte
		if isIndirect(line.Operands[1]) && line.Operands[1] == "(HL)" {
			regCode = 0x06
		} else {
			reg, ok := parseRegister(line.Operands[1])
			if !ok {
				return nil, fmt.Errorf("invalid register")
			}
			regCode, err = encodeReg8(reg)
			if err != nil {
				return nil, err
			}
		}
		
		opcode := baseOpcode | (byte(bitNum) << 3) | regCode
		return []byte{0xCB, opcode}, nil
	}
}

func makeBitIndexEncoder(baseOpcode byte, isIX bool) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 2 {
			return nil, fmt.Errorf("instruction requires 2 operands")
		}
		
		// Parse bit number
		bitNum, err := parseOperandValue(line.Operands[0])
		if err != nil {
			return nil, err
		}
		if bitNum > 7 {
			return nil, fmt.Errorf("bit number must be 0-7")
		}
		
		// Parse index offset
		offset, err := getIndexOffset(line.Operands[1])
		if err != nil {
			return nil, err
		}
		
		prefix := byte(0xDD)
		if !isIX {
			prefix = 0xFD
		}
		
		opcode := baseOpcode | (byte(bitNum) << 3) | 0x06
		return []byte{prefix, 0xCB, byte(offset), opcode}, nil
	}
}

func makeJumpEncoder(opcode byte, isRelative bool) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 1 {
			return nil, fmt.Errorf("instruction requires 1 operand")
		}
		
		addr, err := a.resolveValue(line.Operands[0])
		if err != nil {
			return nil, err
		}
		
		return []byte{opcode, byte(addr), byte(addr >> 8)}, nil
	}
}

func makeCondJumpEncoder(opcode byte) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 2 {
			return nil, fmt.Errorf("instruction requires 2 operands")
		}
		
		// Verify condition matches opcode
		// (In a full implementation, we'd check this)
		
		addr, err := a.resolveValue(line.Operands[1])
		if err != nil {
			return nil, err
		}
		
		return []byte{opcode, byte(addr), byte(addr >> 8)}, nil
	}
}

func makeRelativeEncoder(opcode byte) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 1 {
			return nil, fmt.Errorf("instruction requires 1 operand")
		}
		
		target, err := a.resolveValue(line.Operands[0])
		if err != nil {
			return nil, err
		}
		
		// Calculate relative offset
		offset := int(target) - int(a.currentAddr) - 2
		if offset < -128 || offset > 127 {
			return nil, fmt.Errorf("relative jump out of range: %d", offset)
		}
		
		return []byte{opcode, byte(offset)}, nil
	}
}

func makeCondRelativeEncoder(opcode byte) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 2 {
			return nil, fmt.Errorf("instruction requires 2 operands")
		}
		
		target, err := a.resolveValue(line.Operands[1])
		if err != nil {
			return nil, err
		}
		
		// Calculate relative offset
		offset := int(target) - int(a.currentAddr) - 2
		if offset < -128 || offset > 127 {
			return nil, fmt.Errorf("relative jump out of range: %d", offset)
		}
		
		return []byte{opcode, byte(offset)}, nil
	}
}

func makeCondRetEncoder(opcode byte) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 1 {
			return nil, fmt.Errorf("conditional RET requires 1 operand")
		}
		
		// Verify condition matches opcode
		// (In a full implementation, we'd check this)
		
		return []byte{opcode}, nil
	}
}

func makeRSTEncoder(opcode byte) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 1 {
			return nil, fmt.Errorf("RST requires 1 operand")
		}
		
		// Parse RST vector
		vec, err := a.resolveValue(line.Operands[0])
		if err != nil {
			return nil, err
		}
		
		// Debug output
		fmt.Printf("DEBUG RST: operand='%s', resolved value=%d ($%02X)\n", line.Operands[0], vec, vec)
		
		// Verify it's a valid RST vector
		// RST vectors are at addresses 00h, 08h, 10h, 18h, 20h, 28h, 30h, 38h
		// Accept any base (decimal, hex with $, hex with 0x, etc.)
		validVectors := map[uint16]byte{
			0x00: 0xC7, 0x08: 0xCF, 0x10: 0xD7, 0x18: 0xDF,
			0x20: 0xE7, 0x28: 0xEF, 0x30: 0xF7, 0x38: 0xFF,
		}
		
		if opcode, ok := validVectors[vec]; ok {
			fmt.Printf("DEBUG RST: found opcode %02X for vector %d\n", opcode, vec)
			return []byte{opcode}, nil
		}
		
		return nil, fmt.Errorf("invalid RST vector: %d ($%02X) - valid vectors are 0, 8, 16, 24, 32, 40, 48, 56 (or $00, $08, $10, $18, $20, $28, $30, $38)", vec, vec)
	}
}

func makeStackEncoder(opcode byte, isPush bool) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 1 {
			return nil, fmt.Errorf("PUSH/POP requires 1 operand")
		}
		
		reg, ok := parseRegister(line.Operands[0])
		if !ok {
			return nil, fmt.Errorf("invalid register")
		}
		
		// Map register to opcode
		opcodeMap := map[Register]byte{
			RegBC: 0xC5, RegDE: 0xD5, RegHL: 0xE5, RegAF: 0xF5,
		}
		
		if !isPush {
			opcodeMap = map[Register]byte{
				RegBC: 0xC1, RegDE: 0xD1, RegHL: 0xE1, RegAF: 0xF1,
			}
		}
		
		if op, ok := opcodeMap[reg]; ok {
			return []byte{op}, nil
		}
		
		return nil, fmt.Errorf("invalid register for PUSH/POP")
	}
}

func makeIndexStackEncoder(opcode byte, isPush bool) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 1 {
			return nil, fmt.Errorf("PUSH/POP requires 1 operand")
		}
		
		reg, ok := parseRegister(line.Operands[0])
		if !ok {
			return nil, fmt.Errorf("invalid register")
		}
		
		if reg == RegIX {
			return []byte{0xDD, opcode}, nil
		} else if reg == RegIY {
			return []byte{0xFD, opcode}, nil
		}
		
		return nil, fmt.Errorf("expected IX or IY")
	}
}

func encodeINOUT(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
	if len(line.Operands) != 2 {
		return nil, fmt.Errorf("IN/OUT requires 2 operands")
	}
	
	// Check for IN A, (n) or OUT (n), A
	if line.Mnemonic == "IN" {
		reg, ok := parseRegister(line.Operands[0])
		if ok && reg == RegA && isIndirect(line.Operands[1]) {
			port, err := a.resolveValue(stripIndirect(line.Operands[1]))
			if err != nil {
				return nil, err
			}
			return []byte{0xDB, byte(port)}, nil
		}
	} else if line.Mnemonic == "OUT" {
		reg, ok := parseRegister(line.Operands[1])
		if ok && reg == RegA && isIndirect(line.Operands[0]) {
			port, err := a.resolveValue(stripIndirect(line.Operands[0]))
			if err != nil {
				return nil, err
			}
			return []byte{0xD3, byte(port)}, nil
		}
	}
	
	return nil, fmt.Errorf("invalid IN/OUT instruction")
}

func makeEDIOEncoder(opcode byte, isIN bool) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 2 {
			return nil, fmt.Errorf("IN/OUT requires 2 operands")
		}
		
		// Verify (C) operand
		var portOp string
		if isIN {
			portOp = line.Operands[1]
		} else {
			portOp = line.Operands[0]
		}
		
		if portOp != "(C)" {
			return nil, fmt.Errorf("expected (C) for port")
		}
		
		// For now, assume opcode is correct for the register
		return []byte{0xED, opcode}, nil
	}
}

func makeIMEncoder(opcode byte) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		if len(line.Operands) != 1 {
			return nil, fmt.Errorf("IM requires 1 operand")
		}
		
		mode, err := a.resolveValue(line.Operands[0])
		if err != nil {
			return nil, err
		}
		
		// Map mode to opcode
		opcodeMap := map[uint16]byte{
			0: 0x46, 1: 0x56, 2: 0x5E,
		}
		
		if op, ok := opcodeMap[mode]; ok {
			return []byte{0xED, op}, nil
		}
		
		return nil, fmt.Errorf("invalid interrupt mode: %d", mode)
	}
}

func encodeSpecialLD(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
	if len(line.Operands) != 2 {
		return nil, fmt.Errorf("LD requires 2 operands")
	}
	
	destReg, _ := parseRegister(line.Operands[0])
	srcReg, _ := parseRegister(line.Operands[1])
	
	// LD I, A
	if destReg == RegI && srcReg == RegA {
		return []byte{0xED, 0x47}, nil
	}
	// LD R, A
	if destReg == RegR && srcReg == RegA {
		return []byte{0xED, 0x4F}, nil
	}
	// LD A, I
	if destReg == RegA && srcReg == RegI {
		return []byte{0xED, 0x57}, nil
	}
	// LD A, R
	if destReg == RegA && srcReg == RegR {
		return []byte{0xED, 0x5F}, nil
	}
	
	// Not a special LD - let regular LD handler deal with it
	return encodeLD(a, line, def)
}

func encodeJPIndirect(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
	if len(line.Operands) != 1 {
		return nil, fmt.Errorf("JP requires 1 operand")
	}
	
	if !isIndirect(line.Operands[0]) {
		return nil, fmt.Errorf("expected indirect operand")
	}
	
	inner := stripIndirect(line.Operands[0])
	reg, ok := parseRegister(inner)
	if !ok {
		return nil, fmt.Errorf("invalid register")
	}
	
	switch reg {
	case RegHL:
		return []byte{0xE9}, nil
	case RegIX:
		return []byte{0xDD, 0xE9}, nil
	case RegIY:
		return []byte{0xFD, 0xE9}, nil
	default:
		return nil, fmt.Errorf("JP indirect only supports HL, IX, IY")
	}
}