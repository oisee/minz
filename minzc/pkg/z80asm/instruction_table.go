package z80asm

import (
	"fmt"
	"strings"
)

// OpType defines the type of operand for table-driven encoding
type OpType int

const (
	OpTypeNone      OpType = iota
	OpTypeReg8             // A, B, C, D, E, H, L
	OpTypeReg16            // BC, DE, HL, SP, IX, IY, AF
	OpTypeImm8             // n (8-bit immediate)
	OpTypeImm16            // nn (16-bit immediate)
	OpTypeIndReg           // (HL), (BC), (DE), (SP)
	OpTypeIndImm           // (nn) - memory address
	OpTypeIndIdx           // (IX+d), (IY+d)
	OpTypeRelative         // e (relative jump)
	OpTypeCondition        // NZ, Z, NC, C, PO, PE, P, M
	OpTypeBit              // 0-7 (bit number)
)

// InstructionPattern defines a pattern for matching and encoding instructions
type InstructionPattern struct {
	Mnemonic     string
	Operands     []OperandPattern
	Encoding     []byte
	EncodingFunc func(*Assembler, *InstructionPattern, []interface{}) ([]byte, error)
	Size         int // Size in bytes (0 = calculate from encoding)
	Cycles       int // Clock cycles
}

// OperandPattern defines the pattern for a single operand
type OperandPattern struct {
	Type       OpType
	Constraint string // Specific value required (e.g., "A", "HL", etc.)
}

// Global instruction table - will be populated with all patterns
var instructionTable []InstructionPattern

// Initialize instruction table with all Z80 instructions
// NOTE: ixiyInstructions MUST come before ldInstructions so that
// (IX+d) operands match OpTypeIndIdx before OpTypeIndImm can grab them
func init() {
	instructionTable = append(instructionTable, ixiyInstructions...)
	instructionTable = append(instructionTable, ldInstructions...)
	instructionTable = append(instructionTable, jpInstructions...)
	instructionTable = append(instructionTable, arithmeticInstructions...)
	instructionTable = append(instructionTable, stackInstructions...)
	instructionTable = append(instructionTable, bitInstructions...)
	instructionTable = append(instructionTable, miscInstructions...)
}

// LD instruction patterns - comprehensive coverage
var ldInstructions = []InstructionPattern{
	// ========== 8-bit register to register ==========
	// LD A, r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x7F}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x78}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x79}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x7A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x7B}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x7C}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x7D}},
	
	// LD B, r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x47}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x40}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x41}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x42}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x43}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x44}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x45}},
	
	// LD C, r  
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x4F}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x48}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x49}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x4A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x4B}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x4C}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x4D}},
	
	// LD D, r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x57}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x50}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x51}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x52}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x53}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x54}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x55}},
	
	// LD E, r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x5F}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x58}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x59}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x5A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x5B}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x5C}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x5D}},
	
	// LD H, r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x67}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x60}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x61}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x62}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x63}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x64}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x65}},
	
	// LD L, r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x6F}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x68}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x69}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x6A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x6B}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x6C}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x6D}},
	
	// ========== 8-bit immediate loads ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x3E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x06}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x0E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x16}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x1E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x26}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x2E}},
	
	// ========== (HL) operations ==========
	// LD r, (HL)
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x7E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x46}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x4E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x56}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x5E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x66}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x6E}},
	
	// LD (HL), r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x77}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x70}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x71}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x72}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x73}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x74}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x75}},
	
	// LD (HL), n
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x36}},
	
	// ========== (BC), (DE) operations ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndReg, "(BC)"}}, Encoding: []byte{0x0A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndReg, "(DE)"}}, Encoding: []byte{0x1A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(BC)"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x02}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(DE)"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x12}},
	
	// ========== Special register loads (ED prefix) ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "I"}}, Encoding: []byte{0xED, 0x57}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "R"}}, Encoding: []byte{0xED, 0x5F}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "I"}, {OpTypeReg8, "A"}}, Encoding: []byte{0xED, 0x47}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "R"}, {OpTypeReg8, "A"}}, Encoding: []byte{0xED, 0x4F}},

	// ========== Direct memory operations ==========
	// LD A, (nn)
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0x3A}},
	// LD (nn), A
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg8, "A"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0x32}},
	
	// ========== 16-bit immediate loads ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "BC"}, {OpTypeImm16, ""}}, EncodingFunc: encodeLD16Imm, Encoding: []byte{0x01}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "DE"}, {OpTypeImm16, ""}}, EncodingFunc: encodeLD16Imm, Encoding: []byte{0x11}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeImm16, ""}}, EncodingFunc: encodeLD16Imm, Encoding: []byte{0x21}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "SP"}, {OpTypeImm16, ""}}, EncodingFunc: encodeLD16Imm, Encoding: []byte{0x31}},
	
	// IX/IY immediate loads
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "IX"}, {OpTypeImm16, ""}}, EncodingFunc: encodeLD16Imm, Encoding: []byte{0xDD, 0x21}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "IY"}, {OpTypeImm16, ""}}, EncodingFunc: encodeLD16Imm, Encoding: []byte{0xFD, 0x21}},
	
	// ========== 16-bit memory operations ==========
	// LD HL, (nn)
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0x2A}},
	// LD (nn), HL
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg16, "HL"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0x22}},
	
	// IX/IY memory operations
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "IX"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xDD, 0x2A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg16, "IX"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xDD, 0x22}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "IY"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xFD, 0x2A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg16, "IY"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xFD, 0x22}},
	
	// ED-prefixed 16-bit memory operations
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "BC"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xED, 0x4B}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "DE"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xED, 0x5B}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "SP"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xED, 0x7B}},
	
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg16, "BC"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xED, 0x43}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg16, "DE"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xED, 0x53}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg16, "SP"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xED, 0x73}},
	
	// ========== Special 16-bit transfers ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "SP"}, {OpTypeReg16, "HL"}}, Encoding: []byte{0xF9}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "SP"}, {OpTypeReg16, "IX"}}, Encoding: []byte{0xDD, 0xF9}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "SP"}, {OpTypeReg16, "IY"}}, Encoding: []byte{0xFD, 0xF9}},
}

// Encoding functions for LD instructions
func encodeLD8Imm(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)

	// Find immediate value
	for _, v := range values {
		if val, ok := v.(int); ok && val <= 0xFF {
			result = append(result, byte(val))
			return result, nil
		}
	}

	return nil, fmt.Errorf("no immediate value found")
}

func encodeLD16Imm(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)

	// Find immediate value
	for _, v := range values {
		if val, ok := v.(int); ok {
			result = append(result, a.emitWord(val)...)
			return result, nil
		}
	}

	return nil, fmt.Errorf("no immediate value found")
}

func encodeLDMemDirect(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)

	// Find address value
	for _, v := range values {
		if addr, ok := v.(int); ok {
			result = append(result, a.emitWord(addr)...)
			return result, nil
		}
	}

	return nil, fmt.Errorf("no address value found")
}

// JP/JR instruction patterns
var jpInstructions = []InstructionPattern{
	// Unconditional jumps
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xC3}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0xE9}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeIndReg, "(IX)"}}, Encoding: []byte{0xDD, 0xE9}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeIndReg, "(IY)"}}, Encoding: []byte{0xFD, 0xE9}},
	
	// Conditional jumps
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "NZ"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xC2}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "Z"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xCA}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "NC"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xD2}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "C"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xDA}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "PO"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xE2}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "PE"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xEA}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "P"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xF2}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "M"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xFA}},
	
	// Relative jumps
	{Mnemonic: "JR", Operands: []OperandPattern{{OpTypeRelative, ""}}, EncodingFunc: encodeJRRel, Encoding: []byte{0x18}},
	{Mnemonic: "JR", Operands: []OperandPattern{{OpTypeCondition, "NZ"}, {OpTypeRelative, ""}}, EncodingFunc: encodeJRRel, Encoding: []byte{0x20}},
	{Mnemonic: "JR", Operands: []OperandPattern{{OpTypeCondition, "Z"}, {OpTypeRelative, ""}}, EncodingFunc: encodeJRRel, Encoding: []byte{0x28}},
	{Mnemonic: "JR", Operands: []OperandPattern{{OpTypeCondition, "NC"}, {OpTypeRelative, ""}}, EncodingFunc: encodeJRRel, Encoding: []byte{0x30}},
	{Mnemonic: "JR", Operands: []OperandPattern{{OpTypeCondition, "C"}, {OpTypeRelative, ""}}, EncodingFunc: encodeJRRel, Encoding: []byte{0x38}},
	
	// DJNZ
	{Mnemonic: "DJNZ", Operands: []OperandPattern{{OpTypeRelative, ""}}, EncodingFunc: encodeJRRel, Encoding: []byte{0x10}},

	// JJ - Collapsible jump (collapses to nothing/JR/JP based on distance)
	{Mnemonic: "JJ", Operands: []OperandPattern{{OpTypeImm16, ""}}, EncodingFunc: encodeJJ, Encoding: []byte{0x18, 0xC3}},  // JR/JP opcodes
	{Mnemonic: "JJ", Operands: []OperandPattern{{OpTypeCondition, "NZ"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJJCond, Encoding: []byte{0x20, 0xC2}},
	{Mnemonic: "JJ", Operands: []OperandPattern{{OpTypeCondition, "Z"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJJCond, Encoding: []byte{0x28, 0xCA}},
	{Mnemonic: "JJ", Operands: []OperandPattern{{OpTypeCondition, "NC"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJJCond, Encoding: []byte{0x30, 0xD2}},
	{Mnemonic: "JJ", Operands: []OperandPattern{{OpTypeCondition, "C"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJJCond, Encoding: []byte{0x38, 0xDA}},

	// JJ.L - Likely taken (prefer JP for speed)
	{Mnemonic: "JJ.L", Operands: []OperandPattern{{OpTypeImm16, ""}}, EncodingFunc: encodeJJLikely, Encoding: []byte{0x18, 0xC3}},
	{Mnemonic: "JJ.L", Operands: []OperandPattern{{OpTypeCondition, "NZ"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJJCondLikely, Encoding: []byte{0x20, 0xC2}},
	{Mnemonic: "JJ.L", Operands: []OperandPattern{{OpTypeCondition, "Z"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJJCondLikely, Encoding: []byte{0x28, 0xCA}},
	{Mnemonic: "JJ.L", Operands: []OperandPattern{{OpTypeCondition, "NC"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJJCondLikely, Encoding: []byte{0x30, 0xD2}},
	{Mnemonic: "JJ.L", Operands: []OperandPattern{{OpTypeCondition, "C"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJJCondLikely, Encoding: []byte{0x38, 0xDA}},

	// JJ.U - Unlikely taken (prefer JR for size + speed when not taken)
	{Mnemonic: "JJ.U", Operands: []OperandPattern{{OpTypeImm16, ""}}, EncodingFunc: encodeJJUnlikely, Encoding: []byte{0x18, 0xC3}},
	{Mnemonic: "JJ.U", Operands: []OperandPattern{{OpTypeCondition, "NZ"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJJCondUnlikely, Encoding: []byte{0x20, 0xC2}},
	{Mnemonic: "JJ.U", Operands: []OperandPattern{{OpTypeCondition, "Z"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJJCondUnlikely, Encoding: []byte{0x28, 0xCA}},
	{Mnemonic: "JJ.U", Operands: []OperandPattern{{OpTypeCondition, "NC"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJJCondUnlikely, Encoding: []byte{0x30, 0xD2}},
	{Mnemonic: "JJ.U", Operands: []OperandPattern{{OpTypeCondition, "C"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJJCondUnlikely, Encoding: []byte{0x38, 0xDA}},

	// CALL instructions
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xCD}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "NZ"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xC4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "Z"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xCC}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "NC"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xD4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "C"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xDC}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "PO"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xE4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "PE"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xEC}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "P"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xF4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "M"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xFC}},
	
	// RET instructions
	{Mnemonic: "RET", Operands: []OperandPattern{}, Encoding: []byte{0xC9}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "NZ"}}, Encoding: []byte{0xC0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "Z"}}, Encoding: []byte{0xC8}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "NC"}}, Encoding: []byte{0xD0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "C"}}, Encoding: []byte{0xD8}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "PO"}}, Encoding: []byte{0xE0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "PE"}}, Encoding: []byte{0xE8}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "P"}}, Encoding: []byte{0xF0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "M"}}, Encoding: []byte{0xF8}},
	{Mnemonic: "RETI", Operands: []OperandPattern{}, Encoding: []byte{0xED, 0x4D}},
	{Mnemonic: "RETN", Operands: []OperandPattern{}, Encoding: []byte{0xED, 0x45}},
	
	// RST instructions - use custom encoding function to handle any number base
	{Mnemonic: "RST", Operands: []OperandPattern{{OpTypeImm8, ""}}, EncodingFunc: encodeRST},
	
	// I/O instructions
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeIN},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x78}},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x40}},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x48}},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x50}},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x58}},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x60}},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x68}},
	// IN F,(C) - undocumented, reads port but only sets flags (ED 70)
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "F"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x70}},

	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg8, "A"}}, EncodingFunc: encodeOUT},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "A"}}, Encoding: []byte{0xED, 0x79}},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "B"}}, Encoding: []byte{0xED, 0x41}},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "C"}}, Encoding: []byte{0xED, 0x49}},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "D"}}, Encoding: []byte{0xED, 0x51}},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "E"}}, Encoding: []byte{0xED, 0x59}},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "H"}}, Encoding: []byte{0xED, 0x61}},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "L"}}, Encoding: []byte{0xED, 0x69}},
	// OUT (C),0 - undocumented, outputs zero to port C (ED 71)
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeImm8, ""}}, EncodingFunc: encodeOUT_C_0},
}

func encodeRST(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	// RST n format - n must be 0, 8, 16, 24, 32, 40, 48, or 56
	// These map to opcodes C7, CF, D7, DF, E7, EF, F7, FF

	// Find the vector value
	var vector int
	found := false
	for _, v := range values {
		if val, ok := v.(int); ok {
			vector = val
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("RST requires a vector operand")
	}

	// Map vector to opcode
	validVectors := map[int]byte{
		0x00: 0xC7, 0x08: 0xCF, 0x10: 0xD7, 0x18: 0xDF,
		0x20: 0xE7, 0x28: 0xEF, 0x30: 0xF7, 0x38: 0xFF,
	}

	if opcode, ok := validVectors[vector]; ok {
		return []byte{opcode}, nil
	}

	return nil, fmt.Errorf("invalid RST vector: %d ($%02X) - valid vectors are 0, 8, 16 ($10), 24 ($18), 32 ($20), 40 ($28), 48 ($30), 56 ($38)", vector, vector)
}

func encodeIN(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	// IN A, (n) format
	result := []byte{0xDB} // IN opcode

	// Find port number
	for _, v := range values {
		if val, ok := v.(int); ok {
			if val > 255 {
				return nil, fmt.Errorf("port number out of range: %d", val)
			}
			result = append(result, byte(val))
			return result, nil
		}
	}

	return nil, fmt.Errorf("no port number found")
}

func encodeOUT(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	// OUT (n), A format
	result := []byte{0xD3} // OUT opcode

	// Find port number
	for _, v := range values {
		if val, ok := v.(int); ok {
			if val > 255 {
				return nil, fmt.Errorf("port number out of range: %d", val)
			}
			result = append(result, byte(val))
			return result, nil
		}
	}

	return nil, fmt.Errorf("no port number found")
}

func encodeOUT_C_0(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	// OUT (C), 0 - undocumented, only valid with operand 0
	for _, v := range values {
		if val, ok := v.(int); ok {
			if val != 0 {
				return nil, fmt.Errorf("OUT (C),n only valid with 0, got %d", val)
			}
			return []byte{0xED, 0x71}, nil
		}
	}
	return nil, fmt.Errorf("no immediate value found")
}

// encodeBitOp encodes BIT/SET/RES b, r instructions
// Encoding[0] = base opcode (0x40=BIT, 0x80=RES, 0xC0=SET), Encoding[1] = reg code
func encodeBitOp(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	base := pattern.Encoding[0]
	regCode := pattern.Encoding[1]
	for _, v := range values {
		if bit, ok := v.(uint8); ok {
			opcode := base | (bit << 3) | regCode
			return []byte{0xCB, opcode}, nil
		}
	}
	return nil, fmt.Errorf("no bit number found")
}

// encodeBitIdx encodes BIT/SET/RES b, (IX/IY+d) instructions
// Encoding[0] = prefix (0xDD/0xFD), Encoding[1] = base opcode, Encoding[2] = reg code (0x06 for (HL))
func encodeBitIdx(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	prefix := pattern.Encoding[0]
	base := pattern.Encoding[1]
	regCode := pattern.Encoding[2]
	var bit uint8
	var disp int8
	hasBit := false
	for _, v := range values {
		switch val := v.(type) {
		case uint8:
			bit = val
			hasBit = true
		case int8:
			disp = val
		}
	}
	if !hasBit {
		return nil, fmt.Errorf("no bit number found")
	}
	opcode := base | (bit << 3) | regCode
	return []byte{prefix, 0xCB, byte(disp), opcode}, nil
}

func encodeJPImm(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)

	// Find address value
	for _, v := range values {
		if addr, ok := v.(int); ok {
			result = append(result, a.emitWord(addr)...)
			return result, nil
		}
	}

	return nil, fmt.Errorf("no address value found")
}

func encodeJRRel(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)

	// Find relative offset
	for _, v := range values {
		if offset, ok := v.(int8); ok {
			result = append(result, byte(offset))
			return result, nil
		}
		// Also accept calculated offset from label
		if addr, ok := v.(int); ok {
			// In pass 1, just use a placeholder
			if a.pass == 1 {
				result = append(result, 0) // Placeholder for size calculation
				return result, nil
			}
			// In pass 2, calculate relative offset from current position
			offset := addr - a.currentAddr - 2
			if offset < -128 || offset > 127 {
				// Auto-promote JR to JP when out of range
				var jpOpcode byte
				if len(pattern.Encoding) > 0 {
					switch pattern.Encoding[0] {
					case 0x18:
						jpOpcode = 0xC3 // JR -> JP
					case 0x20:
						jpOpcode = 0xC2 // JR NZ -> JP NZ
					case 0x28:
						jpOpcode = 0xCA // JR Z -> JP Z
					case 0x30:
						jpOpcode = 0xD2 // JR NC -> JP NC
					case 0x38:
						jpOpcode = 0xDA // JR C -> JP C
					default:
						return nil, fmt.Errorf("relative jump out of range: %d", offset)
					}
					return append([]byte{jpOpcode}, a.emitWord(addr)...), nil
				}
				return nil, fmt.Errorf("relative jump out of range: %d", offset)
			}
			result = append(result, byte(offset))
			return result, nil
		}
	}

	return nil, fmt.Errorf("no relative offset found")
}

// ============================================================================
// JJ (Collapsible Jump) Encoding Functions
// ============================================================================
// JJ is a "smart jump" pseudo-instruction that collapses to:
//   - Nothing (0 bytes) when distance = 0
//   - JR (2 bytes) when distance ≤ 127 and appropriate
//   - JP (3 bytes) for longer distances or when speed is preferred
//
// Z80 Branch Timing Analysis:
//   JR taken:     12 T-states
//   JR not taken:  7 T-states
//   JP always:    10 T-states
//
// Decision Matrix:
//   - Balanced mode: Forward jumps use JR (smaller), backward (loops) use JP (faster)
//   - Speed mode: Always JP (consistent 10 T-states)
//   - Size mode: JR when possible, JP when out of range
//   - Likely (.L): Use JP (branch is usually taken, need speed)
//   - Unlikely (.U): Use JR (branch rarely taken, 7 T-states when not taken)
// ============================================================================

// encodeJJ encodes unconditional JJ (collapsible jump)
func encodeJJ(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	// Get target address
	var target int
	for _, v := range values {
		if val, ok := v.(int); ok {
			target = val
			break
		}
	}

	jpOpcode := pattern.Encoding[1]  // 0xC3 for unconditional JP

	// Pass 1: Always use worst-case size (JP = 3 bytes) for label calculation
	// Forward references return 0 in pass 1, so offset calculation would be wrong
	if a.pass == 1 {
		return []byte{jpOpcode, 0x00, 0x00}, nil // Placeholder JP
	}

	// Pass 2+: With multi-pass convergence, we can now optimize JR vs JP
	// Labels will be recalculated if our size changes

	// Calculate offset assuming JR (2 bytes)
	currentPC := a.currentAddr + 2
	offset := int(target) - int(currentPC)

	jrOpcode := pattern.Encoding[0]  // 0x18 for unconditional JR

	switch a.OptMode {
	case OptSpeed:
		// Always use JP for consistent 10 T-state timing
		return append([]byte{jpOpcode}, a.emitWord(target)...), nil

	case OptSize:
		// Use JR when possible, JP only when out of range
		if offset >= -128 && offset <= 127 {
			return []byte{jrOpcode, byte(offset)}, nil
		}
		return append([]byte{jpOpcode}, a.emitWord(target)...), nil

	default: // OptBalanced
		// Forward jumps: JR (smaller, OK for one-time jumps)
		// Backward jumps: JP (faster, loops execute many times)
		if offset >= -128 && offset <= 127 {
			if offset < 0 {
				// Backward jump = likely a loop, prefer JP for speed
				return append([]byte{jpOpcode}, a.emitWord(target)...), nil
			}
			// Forward jump, use JR for size
			return []byte{jrOpcode, byte(offset)}, nil
		}
		// Out of range, must use JP
		return append([]byte{jpOpcode}, a.emitWord(target)...), nil
	}
}

// encodeJJCond encodes conditional JJ (JJ NZ/Z/NC/C)
func encodeJJCond(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	// Get target address (skip condition in values)
	var target int
	for _, v := range values {
		if val, ok := v.(int); ok {
			target = val
			break
		}
	}

	jpOpcode := pattern.Encoding[1]  // Conditional JP opcode

	// Pass 1: Reserve 3 bytes for JP
	if a.pass == 1 {
		return []byte{jpOpcode, 0x00, 0x00}, nil
	}

	// Pass 2+: Optimize with multi-pass convergence
	currentPC := a.currentAddr + 2
	offset := target - currentPC

	jrOpcode := pattern.Encoding[0]  // Conditional JR opcode

	switch a.OptMode {
	case OptSpeed:
		return append([]byte{jpOpcode}, a.emitWord(target)...), nil

	case OptSize:
		if offset >= -128 && offset <= 127 {
			return []byte{jrOpcode, byte(offset)}, nil
		}
		return append([]byte{jpOpcode}, a.emitWord(target)...), nil

	default: // OptBalanced
		if offset >= -128 && offset <= 127 {
			if offset < 0 {
				return append([]byte{jpOpcode}, a.emitWord(target)...), nil
			}
			return []byte{jrOpcode, byte(offset)}, nil
		}
		return append([]byte{jpOpcode}, a.emitWord(target)...), nil
	}
}

// encodeJJLikely encodes JJ.L (likely taken - prefer JP for speed)
func encodeJJLikely(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	var target int
	for _, v := range values {
		if val, ok := v.(int); ok {
			target = val
			break
		}
	}

	jpOpcode := pattern.Encoding[1]

	// Pass 1: Always use JP size
	if a.pass == 1 {
		return []byte{jpOpcode, 0x00, 0x00}, nil
	}

	// Pass 2: Likely = branch usually taken, use JP for 10 T-state consistency
	return append([]byte{jpOpcode}, a.emitWord(target)...), nil
}

// encodeJJUnlikely encodes JJ.U (unlikely taken - prefer JR for speed when not taken)
// With multi-pass convergence, emits JR when possible for better not-taken timing (7 vs 10 T)
func encodeJJUnlikely(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	var target int
	for _, v := range values {
		if val, ok := v.(int); ok {
			target = val
			break
		}
	}

	jpOpcode := pattern.Encoding[1]

	// Pass 1: Reserve 3 bytes
	if a.pass == 1 {
		return []byte{jpOpcode, 0x00, 0x00}, nil
	}

	// Pass 2+: Prefer JR for unlikely branches (7 T-states when not taken vs 10)
	currentPC := a.currentAddr + 2
	offset := target - currentPC

	jrOpcode := pattern.Encoding[0]

	// Unlikely = branch rarely taken, prefer JR
	if offset >= -128 && offset <= 127 {
		return []byte{jrOpcode, byte(offset)}, nil
	}
	// Out of range, must use JP
	return append([]byte{jpOpcode}, a.emitWord(target)...), nil
}

// encodeJJCondLikely encodes conditional JJ.L
func encodeJJCondLikely(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	var target int
	for _, v := range values {
		if val, ok := v.(int); ok {
			target = val
			break
		}
	}

	jpOpcode := pattern.Encoding[1]

	// Pass 1: Always use JP size
	if a.pass == 1 {
		return []byte{jpOpcode, 0x00, 0x00}, nil
	}

	// Pass 2: Likely = use JP for speed
	return append([]byte{jpOpcode}, a.emitWord(target)...), nil
}

// encodeJJCondUnlikely encodes conditional JJ.U
// With multi-pass convergence, emits JR for better not-taken timing
func encodeJJCondUnlikely(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	var target int
	for _, v := range values {
		if val, ok := v.(int); ok {
			target = val
			break
		}
	}

	jpOpcode := pattern.Encoding[1]

	// Pass 1: Reserve 3 bytes
	if a.pass == 1 {
		return []byte{jpOpcode, 0x00, 0x00}, nil
	}

	// Pass 2+: Prefer JR for unlikely branches
	currentPC := a.currentAddr + 2
	offset := target - currentPC

	jrOpcode := pattern.Encoding[0]

	if offset >= -128 && offset <= 127 {
		return []byte{jrOpcode, byte(offset)}, nil
	}
	return append([]byte{jpOpcode}, a.emitWord(target)...), nil
}

// Arithmetic instruction patterns
var arithmeticInstructions = []InstructionPattern{
	// ADD A, r
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x87}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x80}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x81}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x82}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x83}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x84}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x85}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x86}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xC6}},
	
	// ADC A, r
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x8F}},
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x88}},
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x89}},
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x8A}},
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x8B}},
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x8C}},
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x8D}},
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x8E}},
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xCE}},

	// SBC A, r
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x9F}},
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x98}},
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x99}},
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x9A}},
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x9B}},
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x9C}},
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x9D}},
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x9E}},
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xDE}},

	// ADC HL, rr (ED prefix)
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeReg16, "BC"}}, Encoding: []byte{0xED, 0x4A}},
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeReg16, "DE"}}, Encoding: []byte{0xED, 0x5A}},
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeReg16, "HL"}}, Encoding: []byte{0xED, 0x6A}},
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeReg16, "SP"}}, Encoding: []byte{0xED, 0x7A}},

	// SBC HL, rr (ED prefix)
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeReg16, "BC"}}, Encoding: []byte{0xED, 0x42}},
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeReg16, "DE"}}, Encoding: []byte{0xED, 0x52}},
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeReg16, "HL"}}, Encoding: []byte{0xED, 0x62}},
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeReg16, "SP"}}, Encoding: []byte{0xED, 0x72}},

	// ADD HL, rr
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeReg16, "BC"}}, Encoding: []byte{0x09}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeReg16, "DE"}}, Encoding: []byte{0x19}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeReg16, "HL"}}, Encoding: []byte{0x29}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeReg16, "SP"}}, Encoding: []byte{0x39}},

	// SUB r
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0x97}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0x90}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0x91}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0x92}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0x93}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0x94}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0x95}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x96}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xD6}},
	
	// INC/DEC
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0x3C}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0x04}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0x0C}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0x14}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0x1C}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0x24}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0x2C}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x34}},
	
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0x3D}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0x05}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0x0D}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0x15}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0x1D}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0x25}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0x2D}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x35}},
	
	// 16-bit INC/DEC
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg16, "BC"}}, Encoding: []byte{0x03}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg16, "DE"}}, Encoding: []byte{0x13}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg16, "HL"}}, Encoding: []byte{0x23}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg16, "SP"}}, Encoding: []byte{0x33}},
	
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg16, "BC"}}, Encoding: []byte{0x0B}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg16, "DE"}}, Encoding: []byte{0x1B}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg16, "HL"}}, Encoding: []byte{0x2B}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg16, "SP"}}, Encoding: []byte{0x3B}},
}

func encodeArithImm(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)

	// Find immediate value
	for _, v := range values {
		if val, ok := v.(int); ok && val <= 0xFF {
			result = append(result, byte(val))
			return result, nil
		}
	}

	return nil, fmt.Errorf("no immediate value found")
}

// Stack instruction patterns
var stackInstructions = []InstructionPattern{
	{Mnemonic: "PUSH", Operands: []OperandPattern{{OpTypeReg16, "AF"}}, Encoding: []byte{0xF5}},
	{Mnemonic: "PUSH", Operands: []OperandPattern{{OpTypeReg16, "BC"}}, Encoding: []byte{0xC5}},
	{Mnemonic: "PUSH", Operands: []OperandPattern{{OpTypeReg16, "DE"}}, Encoding: []byte{0xD5}},
	{Mnemonic: "PUSH", Operands: []OperandPattern{{OpTypeReg16, "HL"}}, Encoding: []byte{0xE5}},
	{Mnemonic: "PUSH", Operands: []OperandPattern{{OpTypeReg16, "IX"}}, Encoding: []byte{0xDD, 0xE5}},
	{Mnemonic: "PUSH", Operands: []OperandPattern{{OpTypeReg16, "IY"}}, Encoding: []byte{0xFD, 0xE5}},
	
	{Mnemonic: "POP", Operands: []OperandPattern{{OpTypeReg16, "AF"}}, Encoding: []byte{0xF1}},
	{Mnemonic: "POP", Operands: []OperandPattern{{OpTypeReg16, "BC"}}, Encoding: []byte{0xC1}},
	{Mnemonic: "POP", Operands: []OperandPattern{{OpTypeReg16, "DE"}}, Encoding: []byte{0xD1}},
	{Mnemonic: "POP", Operands: []OperandPattern{{OpTypeReg16, "HL"}}, Encoding: []byte{0xE1}},
	{Mnemonic: "POP", Operands: []OperandPattern{{OpTypeReg16, "IX"}}, Encoding: []byte{0xDD, 0xE1}},
	{Mnemonic: "POP", Operands: []OperandPattern{{OpTypeReg16, "IY"}}, Encoding: []byte{0xFD, 0xE1}},
}

// Bit operation patterns
var bitInstructions = []InstructionPattern{
	// SLL (Shift Left Logical) - undocumented CB 30-37
	{Mnemonic: "SLL", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0xCB, 0x30}},
	{Mnemonic: "SLL", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0xCB, 0x31}},
	{Mnemonic: "SLL", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0xCB, 0x32}},
	{Mnemonic: "SLL", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0xCB, 0x33}},
	{Mnemonic: "SLL", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0xCB, 0x34}},
	{Mnemonic: "SLL", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0xCB, 0x35}},
	{Mnemonic: "SLL", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0xCB, 0x36}},
	{Mnemonic: "SLL", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0xCB, 0x37}},

	// BIT b, r — CB (0x40 | bit<<3 | reg)
	{Mnemonic: "BIT", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "B"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x40, 0x00}},
	{Mnemonic: "BIT", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "C"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x40, 0x01}},
	{Mnemonic: "BIT", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "D"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x40, 0x02}},
	{Mnemonic: "BIT", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "E"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x40, 0x03}},
	{Mnemonic: "BIT", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "H"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x40, 0x04}},
	{Mnemonic: "BIT", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "L"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x40, 0x05}},
	{Mnemonic: "BIT", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeIndReg, "(HL)"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x40, 0x06}},
	{Mnemonic: "BIT", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "A"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x40, 0x07}},
	{Mnemonic: "BIT", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeBitIdx, Encoding: []byte{0xDD, 0x40, 0x06}},
	{Mnemonic: "BIT", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeBitIdx, Encoding: []byte{0xFD, 0x40, 0x06}},
	// RES b, r — CB (0x80 | bit<<3 | reg)
	{Mnemonic: "RES", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "B"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x80, 0x00}},
	{Mnemonic: "RES", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "C"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x80, 0x01}},
	{Mnemonic: "RES", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "D"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x80, 0x02}},
	{Mnemonic: "RES", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "E"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x80, 0x03}},
	{Mnemonic: "RES", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "H"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x80, 0x04}},
	{Mnemonic: "RES", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "L"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x80, 0x05}},
	{Mnemonic: "RES", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeIndReg, "(HL)"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x80, 0x06}},
	{Mnemonic: "RES", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "A"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0x80, 0x07}},
	{Mnemonic: "RES", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeBitIdx, Encoding: []byte{0xDD, 0x80, 0x06}},
	{Mnemonic: "RES", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeBitIdx, Encoding: []byte{0xFD, 0x80, 0x06}},
	// SET b, r — CB (0xC0 | bit<<3 | reg)
	{Mnemonic: "SET", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "B"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0xC0, 0x00}},
	{Mnemonic: "SET", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "C"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0xC0, 0x01}},
	{Mnemonic: "SET", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "D"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0xC0, 0x02}},
	{Mnemonic: "SET", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "E"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0xC0, 0x03}},
	{Mnemonic: "SET", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "H"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0xC0, 0x04}},
	{Mnemonic: "SET", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "L"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0xC0, 0x05}},
	{Mnemonic: "SET", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeIndReg, "(HL)"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0xC0, 0x06}},
	{Mnemonic: "SET", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeReg8, "A"}}, EncodingFunc: encodeBitOp, Encoding: []byte{0xC0, 0x07}},
	{Mnemonic: "SET", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeBitIdx, Encoding: []byte{0xDD, 0xC0, 0x06}},
	{Mnemonic: "SET", Operands: []OperandPattern{{OpTypeBit, ""}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeBitIdx, Encoding: []byte{0xFD, 0xC0, 0x06}},

	// AND
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0xA7}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0xA0}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0xA1}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0xA2}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0xA3}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0xA4}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0xA5}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0xA6}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xE6}},
	
	// OR
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0xB7}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0xB0}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0xB1}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0xB2}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0xB3}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0xB4}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0xB5}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0xB6}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xF6}},
	
	// XOR
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0xAF}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0xA8}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0xA9}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0xAA}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0xAB}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0xAC}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0xAD}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0xAE}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xEE}},
	
	// CP
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0xBF}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0xB8}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0xB9}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0xBA}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0xBB}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0xBC}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0xBD}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0xBE}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xFE}},
}

// Miscellaneous instruction patterns
var miscInstructions = []InstructionPattern{
	// Basic control
	{Mnemonic: "NOP", Operands: []OperandPattern{}, Encoding: []byte{0x00}},
	{Mnemonic: "HALT", Operands: []OperandPattern{}, Encoding: []byte{0x76}},
	{Mnemonic: "DI", Operands: []OperandPattern{}, Encoding: []byte{0xF3}},
	{Mnemonic: "EI", Operands: []OperandPattern{}, Encoding: []byte{0xFB}},
	
	// Returns
	{Mnemonic: "RET", Operands: []OperandPattern{}, Encoding: []byte{0xC9}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "NZ"}}, Encoding: []byte{0xC0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "Z"}}, Encoding: []byte{0xC8}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "NC"}}, Encoding: []byte{0xD0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "C"}}, Encoding: []byte{0xD8}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "PO"}}, Encoding: []byte{0xE0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "PE"}}, Encoding: []byte{0xE8}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "P"}}, Encoding: []byte{0xF0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "M"}}, Encoding: []byte{0xF8}},
	
	// Calls
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xCD}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "NZ"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xC4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "Z"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xCC}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "NC"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xD4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "C"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xDC}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "PO"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xE4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "PE"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xEC}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "P"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xF4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "M"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xFC}},
	
	// Exchanges
	{Mnemonic: "EX", Operands: []OperandPattern{{OpTypeReg16, "DE"}, {OpTypeReg16, "HL"}}, Encoding: []byte{0xEB}},
	{Mnemonic: "EX", Operands: []OperandPattern{{OpTypeReg16, "AF"}, {OpTypeReg16, "AF'"}}, Encoding: []byte{0x08}},
	{Mnemonic: "EX", Operands: []OperandPattern{{OpTypeIndReg, "(SP)"}, {OpTypeReg16, "HL"}}, Encoding: []byte{0xE3}},
	{Mnemonic: "EX", Operands: []OperandPattern{{OpTypeIndReg, "(SP)"}, {OpTypeReg16, "IX"}}, Encoding: []byte{0xDD, 0xE3}},
	{Mnemonic: "EX", Operands: []OperandPattern{{OpTypeIndReg, "(SP)"}, {OpTypeReg16, "IY"}}, Encoding: []byte{0xFD, 0xE3}},
	{Mnemonic: "EXX", Operands: []OperandPattern{}, Encoding: []byte{0xD9}},
	
	// Rotates
	{Mnemonic: "RLCA", Operands: []OperandPattern{}, Encoding: []byte{0x07}},
	{Mnemonic: "RLA", Operands: []OperandPattern{}, Encoding: []byte{0x17}},
	{Mnemonic: "RRCA", Operands: []OperandPattern{}, Encoding: []byte{0x0F}},
	{Mnemonic: "RRA", Operands: []OperandPattern{}, Encoding: []byte{0x1F}},
	
	// Misc
	{Mnemonic: "DAA", Operands: []OperandPattern{}, Encoding: []byte{0x27}},
	{Mnemonic: "CPL", Operands: []OperandPattern{}, Encoding: []byte{0x2F}},
	{Mnemonic: "NEG", Operands: []OperandPattern{}, Encoding: []byte{0xED, 0x44}},
	{Mnemonic: "CCF", Operands: []OperandPattern{}, Encoding: []byte{0x3F}},
	{Mnemonic: "SCF", Operands: []OperandPattern{}, Encoding: []byte{0x37}},
}

func encodeCALL(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)

	// Find address value
	for _, v := range values {
		if addr, ok := v.(int); ok {
			result = append(result, a.emitWord(addr)...)
			return result, nil
		}
	}

	return nil, fmt.Errorf("no address value found")
}

// ========================================================================
// IX/IY indexed addressing mode instructions
// ========================================================================

// encodeIndIdxDisp appends the index displacement to the base encoding
// Used for: LD r,(IX+d), LD (IX+d),r, ALU A,(IX+d), INC/DEC (IX+d)
func encodeIndIdxDisp(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)

	for _, v := range values {
		if d, ok := v.(int8); ok {
			result = append(result, byte(d))
			return result, nil
		}
	}
	return nil, fmt.Errorf("no index displacement found")
}

// encodeIndIdxDispImm appends displacement then immediate byte
// Used for: LD (IX+d), n
func encodeIndIdxDispImm(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)

	var disp int8
	var imm int
	foundDisp, foundImm := false, false

	for _, v := range values {
		switch val := v.(type) {
		case int8:
			if !foundDisp {
				disp = val
				foundDisp = true
			}
		case int:
			if !foundImm {
				imm = val
				foundImm = true
			}
		}
	}

	if !foundDisp {
		return nil, fmt.Errorf("no index displacement found")
	}
	if !foundImm {
		return nil, fmt.Errorf("no immediate value found")
	}

	result = append(result, byte(disp), byte(imm))
	return result, nil
}

// IX/IY indexed instruction patterns
var ixiyInstructions = []InstructionPattern{
	// ========== LD r, (IX+d) ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x7E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x46}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x4E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x56}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x5E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x66}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x6E}},

	// ========== LD r, (IY+d) ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x7E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x46}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x4E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x56}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x5E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x66}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x6E}},

	// ========== LD (IX+d), r ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}, {OpTypeReg8, "A"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x77}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}, {OpTypeReg8, "B"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x70}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}, {OpTypeReg8, "C"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x71}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}, {OpTypeReg8, "D"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x72}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}, {OpTypeReg8, "E"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x73}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}, {OpTypeReg8, "H"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x74}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}, {OpTypeReg8, "L"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x75}},

	// ========== LD (IY+d), r ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}, {OpTypeReg8, "A"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x77}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}, {OpTypeReg8, "B"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x70}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}, {OpTypeReg8, "C"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x71}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}, {OpTypeReg8, "D"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x72}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}, {OpTypeReg8, "E"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x73}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}, {OpTypeReg8, "H"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x74}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}, {OpTypeReg8, "L"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x75}},

	// ========== LD (IX+d), n ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}, {OpTypeImm8, ""}}, EncodingFunc: encodeIndIdxDispImm, Encoding: []byte{0xDD, 0x36}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}, {OpTypeImm8, ""}}, EncodingFunc: encodeIndIdxDispImm, Encoding: []byte{0xFD, 0x36}},

	// ========== ALU A, (IX+d) / (IY+d) — two-operand forms ==========
	// ADD A, (IX+d) / (IY+d)
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x86}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x86}},
	// ADC A, (IX+d) / (IY+d)
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x8E}},
	{Mnemonic: "ADC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x8E}},
	// SBC A, (IX+d) / (IY+d)
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x9E}},
	{Mnemonic: "SBC", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x9E}},

	// ========== ALU (IX+d) / (IY+d) — one-operand forms ==========
	// SUB (IX+d) / (IY+d)
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x96}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x96}},
	// AND (IX+d) / (IY+d)
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0xA6}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0xA6}},
	// XOR (IX+d) / (IY+d)
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0xAE}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0xAE}},
	// OR (IX+d) / (IY+d)
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0xB6}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0xB6}},
	// CP (IX+d) / (IY+d)
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0xBE}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0xBE}},

	// ========== INC/DEC (IX+d) / (IY+d) ==========
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x34}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x34}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeIndIdx, "IX"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xDD, 0x35}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeIndIdx, "IY"}}, EncodingFunc: encodeIndIdxDisp, Encoding: []byte{0xFD, 0x35}},

	// ========== INC/DEC IX/IY 16-bit ==========
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg16, "IX"}}, Encoding: []byte{0xDD, 0x23}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg16, "IY"}}, Encoding: []byte{0xFD, 0x23}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg16, "IX"}}, Encoding: []byte{0xDD, 0x2B}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg16, "IY"}}, Encoding: []byte{0xFD, 0x2B}},

	// ========== ADD IX/IY, rr ==========
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg16, "IX"}, {OpTypeReg16, "BC"}}, Encoding: []byte{0xDD, 0x09}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg16, "IX"}, {OpTypeReg16, "DE"}}, Encoding: []byte{0xDD, 0x19}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg16, "IX"}, {OpTypeReg16, "IX"}}, Encoding: []byte{0xDD, 0x29}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg16, "IX"}, {OpTypeReg16, "SP"}}, Encoding: []byte{0xDD, 0x39}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg16, "IY"}, {OpTypeReg16, "BC"}}, Encoding: []byte{0xFD, 0x09}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg16, "IY"}, {OpTypeReg16, "DE"}}, Encoding: []byte{0xFD, 0x19}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg16, "IY"}, {OpTypeReg16, "IY"}}, Encoding: []byte{0xFD, 0x29}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg16, "IY"}, {OpTypeReg16, "SP"}}, Encoding: []byte{0xFD, 0x39}},
}

// GetInstructionPatterns returns all patterns for a given mnemonic
func GetInstructionPatterns(mnemonic string) []InstructionPattern {
	mnemonic = strings.ToUpper(mnemonic)
	var patterns []InstructionPattern
	
	for _, pattern := range instructionTable {
		if pattern.Mnemonic == mnemonic {
			patterns = append(patterns, pattern)
		}
	}
	
	return patterns
}