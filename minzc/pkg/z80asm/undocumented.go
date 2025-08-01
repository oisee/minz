package z80asm

import (
	"fmt"
)

// registerUndocumentedInstructions adds all undocumented Z80 instructions
func registerUndocumentedInstructions() {
	// SLL (Shift Left Logical) - undocumented
	registerSLL()
	
	// IX/IY half register operations
	registerIXHalfOps()
	registerIYHalfOps()
	
	// Undocumented ED instructions
	registerUndocumentedED()
	
	// Undocumented bit operations with IX/IY
	registerUndocumentedIXBit()
	registerUndocumentedIYBit()
	
	// Other undocumented instructions
	registerMiscUndocumented()
}

// registerSLL registers the undocumented SLL instruction
func registerSLL() {
	// SLL r - Shift Left Logical (undocumented)
	registers := []struct {
		name string
		reg  Register
		code byte
	}{
		{"B", RegB, 0x30},
		{"C", RegC, 0x31},
		{"D", RegD, 0x32},
		{"E", RegE, 0x33},
		{"H", RegH, 0x34},
		{"L", RegL, 0x35},
		{"(HL)", RegNone, 0x36},
		{"A", RegA, 0x37},
	}
	
	for _, r := range registers {
		def := &InstructionDef{
			Mnemonic:     "SLL",
			Operands:     []OperandType{OpReg8},
			Undocumented: true,
			Size:         2,
			Encoder: func(code byte) EncoderFunc {
				return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
					return []byte{PrefixCB, code}, nil
				}
			}(r.code),
		}
		
		// Special handling for (HL)
		if r.name == "(HL)" {
			def.Operands = []OperandType{OpRegIndirect}
		}
		
		addInstruction("SLL", def)
	}
	
	// SLL (IX+d)
	addInstruction("SLL", &InstructionDef{
		Mnemonic:     "SLL",
		Operands:     []OperandType{OpIXOffset},
		Undocumented: true,
		Size:         4,
		Encoder:      encodeSLLIndex,
	})
	
	// SLL (IY+d)
	addInstruction("SLL", &InstructionDef{
		Mnemonic:     "SLL",
		Operands:     []OperandType{OpIYOffset},
		Undocumented: true,
		Size:         4,
		Encoder:      encodeSLLIndex,
	})
}

// encodeSLLIndex encodes SLL (IX/IY+d)
func encodeSLLIndex(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
	operand := line.Operands[0]
	
	if isIndexedOperand(operand, "IX") {
		offset, err := getIndexOffset(operand)
		if err != nil {
			return nil, err
		}
		return []byte{0xDD, 0xCB, byte(offset), 0x36}, nil
	}
	
	if isIndexedOperand(operand, "IY") {
		offset, err := getIndexOffset(operand)
		if err != nil {
			return nil, err
		}
		return []byte{0xFD, 0xCB, byte(offset), 0x36}, nil
	}
	
	return nil, fmt.Errorf("invalid indexed operand for SLL")
}

// registerIXHalfOps registers operations on IXH and IXL
func registerIXHalfOps() {
	// INC/DEC IXH/IXL
	addInstruction("INC", &InstructionDef{
		Mnemonic:     "INC",
		Operands:     []OperandType{OpReg8},
		Undocumented: true,
		Size:         2,
		Encoder:      encodeIXHalfInc,
	})
	
	addInstruction("DEC", &InstructionDef{
		Mnemonic:     "DEC",
		Operands:     []OperandType{OpReg8},
		Undocumented: true,
		Size:         2,
		Encoder:      encodeIXHalfDec,
	})
	
	// Arithmetic with IXH/IXL
	arithmeticOps := []string{"ADD", "ADC", "SUB", "SBC", "AND", "XOR", "OR", "CP"}
	for _, op := range arithmeticOps {
		addInstruction(op, &InstructionDef{
			Mnemonic:     op,
			Operands:     []OperandType{OpReg8},
			Undocumented: true,
			Size:         2,
			Encoder:      makeIXHalfArithEncoder(op),
		})
		
		// Also register A, IXH/IXL forms
		addInstruction(op, &InstructionDef{
			Mnemonic:     op,
			Operands:     []OperandType{OpReg8, OpReg8},
			Undocumented: true,
			Size:         2,
			Encoder:      makeIXHalfArithEncoder(op),
		})
	}
}

// registerIYHalfOps registers operations on IYH and IYL
func registerIYHalfOps() {
	// INC/DEC IYH/IYL
	addInstruction("INC", &InstructionDef{
		Mnemonic:     "INC",
		Operands:     []OperandType{OpReg8},
		Undocumented: true,
		Size:         2,
		Encoder:      encodeIYHalfInc,
	})
	
	addInstruction("DEC", &InstructionDef{
		Mnemonic:     "DEC",
		Operands:     []OperandType{OpReg8},
		Undocumented: true,
		Size:         2,
		Encoder:      encodeIYHalfDec,
	})
	
	// Arithmetic with IYH/IYL
	arithmeticOps := []string{"ADD", "ADC", "SUB", "SBC", "AND", "XOR", "OR", "CP"}
	for _, op := range arithmeticOps {
		addInstruction(op, &InstructionDef{
			Mnemonic:     op,
			Operands:     []OperandType{OpReg8},
			Undocumented: true,
			Size:         2,
			Encoder:      makeIYHalfArithEncoder(op),
		})
		
		// Also register A, IYH/IYL forms
		addInstruction(op, &InstructionDef{
			Mnemonic:     op,
			Operands:     []OperandType{OpReg8, OpReg8},
			Undocumented: true,
			Size:         2,
			Encoder:      makeIYHalfArithEncoder(op),
		})
	}
}

// Encoder functions for IX/IY half registers

func encodeIXHalfInc(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
	reg, _ := parseRegister(line.Operands[0])
	
	switch reg {
	case RegIXH:
		return []byte{0xDD, 0x24}, nil // INC IXH
	case RegIXL:
		return []byte{0xDD, 0x2C}, nil // INC IXL
	}
	
	return nil, fmt.Errorf("not an IX half register")
}

func encodeIXHalfDec(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
	reg, _ := parseRegister(line.Operands[0])
	
	switch reg {
	case RegIXH:
		return []byte{0xDD, 0x25}, nil // DEC IXH
	case RegIXL:
		return []byte{0xDD, 0x2D}, nil // DEC IXL
	}
	
	return nil, fmt.Errorf("not an IX half register")
}

func encodeIYHalfInc(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
	reg, _ := parseRegister(line.Operands[0])
	
	switch reg {
	case RegIYH:
		return []byte{0xFD, 0x24}, nil // INC IYH
	case RegIYL:
		return []byte{0xFD, 0x2C}, nil // INC IYL
	}
	
	return nil, fmt.Errorf("not an IY half register")
}

func encodeIYHalfDec(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
	reg, _ := parseRegister(line.Operands[0])
	
	switch reg {
	case RegIYH:
		return []byte{0xFD, 0x25}, nil // DEC IYH
	case RegIYL:
		return []byte{0xFD, 0x2D}, nil // DEC IYL
	}
	
	return nil, fmt.Errorf("not an IY half register")
}

// makeIXHalfArithEncoder creates arithmetic encoders for IX half registers
func makeIXHalfArithEncoder(op string) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		var srcReg Register
		
		// Handle both "ADD IXH" and "ADD A, IXH" forms
		if len(line.Operands) == 1 {
			srcReg, _ = parseRegister(line.Operands[0])
		} else if len(line.Operands) == 2 {
			srcReg, _ = parseRegister(line.Operands[1])
		}
		
		// Check if it's an IX half register
		var regCode byte
		switch srcReg {
		case RegIXH:
			regCode = 0x04 // H position
		case RegIXL:
			regCode = 0x05 // L position
		default:
			return nil, fmt.Errorf("not an IX half register")
		}
		
		// Get base opcode for operation
		var baseOp byte
		switch op {
		case "ADD":
			baseOp = 0x80
		case "ADC":
			baseOp = 0x88
		case "SUB":
			baseOp = 0x90
		case "SBC":
			baseOp = 0x98
		case "AND":
			baseOp = 0xA0
		case "XOR":
			baseOp = 0xA8
		case "OR":
			baseOp = 0xB0
		case "CP":
			baseOp = 0xB8
		}
		
		return []byte{0xDD, baseOp | regCode}, nil
	}
}

// makeIYHalfArithEncoder creates arithmetic encoders for IY half registers
func makeIYHalfArithEncoder(op string) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		var srcReg Register
		
		// Handle both "ADD IYH" and "ADD A, IYH" forms
		if len(line.Operands) == 1 {
			srcReg, _ = parseRegister(line.Operands[0])
		} else if len(line.Operands) == 2 {
			srcReg, _ = parseRegister(line.Operands[1])
		}
		
		// Check if it's an IY half register
		var regCode byte
		switch srcReg {
		case RegIYH:
			regCode = 0x04 // H position
		case RegIYL:
			regCode = 0x05 // L position
		default:
			return nil, fmt.Errorf("not an IY half register")
		}
		
		// Get base opcode for operation
		var baseOp byte
		switch op {
		case "ADD":
			baseOp = 0x80
		case "ADC":
			baseOp = 0x88
		case "SUB":
			baseOp = 0x90
		case "SBC":
			baseOp = 0x98
		case "AND":
			baseOp = 0xA0
		case "XOR":
			baseOp = 0xA8
		case "OR":
			baseOp = 0xB0
		case "CP":
			baseOp = 0xB8
		}
		
		return []byte{0xFD, baseOp | regCode}, nil
	}
}

// registerUndocumentedED registers undocumented ED prefix instructions
func registerUndocumentedED() {
	// OUT (C), 0 - outputs zero to port C
	addInstruction("OUT", &InstructionDef{
		Mnemonic:     "OUT",
		Operands:     []OperandType{OpRegIndirect, OpImm8},
		Undocumented: true,
		Size:         2,
		Encoder: func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
			// Check for OUT (C), 0
			if line.Operands[0] == "(C)" && line.Operands[1] == "0" {
				return []byte{0xED, 0x71}, nil
			}
			return nil, fmt.Errorf("not OUT (C), 0")
		},
	})
	
	// Duplicate NEG instructions at various positions
	negOpcodes := []byte{0x44, 0x4C, 0x54, 0x5C, 0x64, 0x6C, 0x74, 0x7C}
	for _, opcode := range negOpcodes[1:] { // Skip first one (documented)
		addInstruction("NEG", &InstructionDef{
			Mnemonic:     "NEG",
			Operands:     []OperandType{},
			Undocumented: true,
			Size:         2,
			Encoder:      encodeEDPrefix(opcode),
		})
	}
}

// registerUndocumentedIXBit registers undocumented bit operations with IX
func registerUndocumentedIXBit() {
	// Undocumented: bit operations on (IX+d) also affect undocumented registers
	// These are complex multi-byte sequences that need special handling
	
	// For now, we'll just ensure the standard IX bit operations work
	// The truly undocumented behavior (side effects) would need emulator support
}

// registerUndocumentedIYBit registers undocumented bit operations with IY
func registerUndocumentedIYBit() {
	// Similar to IX - the undocumented behavior is about side effects
	// Standard operations are handled by normal bit instruction registration
}

// registerMiscUndocumented registers other miscellaneous undocumented instructions
func registerMiscUndocumented() {
	// Some undocumented NOPs in ED space
	undocNops := []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		// ... many more in ED space act as NOPs
	}
	
	for _, opcode := range undocNops {
		addInstruction("NOP", &InstructionDef{
			Mnemonic:     "NOP",
			Operands:     []OperandType{},
			Undocumented: true,
			Size:         2,
			Encoder:      encodeEDPrefix(opcode),
		})
	}
}

// addInstruction is a helper to add instruction definitions
func addInstruction(mnemonic string, def *InstructionDef) {
	if instructionTable[mnemonic] == nil {
		instructionTable[mnemonic] = make([]*InstructionDef, 0)
	}
	instructionTable[mnemonic] = append(instructionTable[mnemonic], def)
}